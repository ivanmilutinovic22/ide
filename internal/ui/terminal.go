package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"

	"ide/internal/config"
	"ide/internal/tmux"
)

// Attribute bitmask constants matching vt10x internals.
const (
	vtAttrReverse   = 1 << 0
	vtAttrUnderline = 1 << 1
	vtAttrBold      = 1 << 2
	vtAttrGfx       = 1 << 3
	vtAttrItalic    = 1 << 4
	vtAttrBlink     = 1 << 5
)

// Color sentinel values from vt10x.
const (
	vtDefaultFG vt10x.Color = 1<<24 + 0
	vtDefaultBG vt10x.Color = 1<<24 + 1
)

// EmbeddedTerminal manages a PTY running tmux attach with a virtual terminal emulator.
type EmbeddedTerminal struct {
	mu       sync.Mutex
	vt       vt10x.Terminal
	ptmx     *os.File
	cmd      *exec.Cmd
	cols     int
	rows     int
	session  string
	window   string
	closed   bool
	fgSGR    map[vt10x.Color]string
	bgSGR    map[vt10x.Color]string
}

const sgrCacheCap = 4096

// ptyReadMsg signals that new PTY output was processed into the virtual terminal.
type ptyReadMsg struct{}

// ptyEOFMsg signals that the PTY was closed.
type ptyEOFMsg struct{ err error }

// terminalSessionReadyMsg signals that a session has been ensured and
// the terminal mode can now be activated.
type terminalSessionReadyMsg struct {
	err error
}

func newEmbeddedTerminal(cols, rows int) *EmbeddedTerminal {
	return &EmbeddedTerminal{
		cols:  cols,
		rows:  rows,
		fgSGR: make(map[vt10x.Color]string),
		bgSGR: make(map[vt10x.Color]string),
	}
}

// Attach starts a PTY running tmux attach-session for the given target.
func (et *EmbeddedTerminal) Attach(session, window string) error {
	et.Close()
	et.mu.Lock()
	defer et.mu.Unlock()

	et.vt = vt10x.New(vt10x.WithSize(et.cols, et.rows))
	et.session = session
	et.window = window
	et.closed = false
	if et.fgSGR == nil {
		et.fgSGR = make(map[vt10x.Color]string)
	}
	if et.bgSGR == nil {
		et.bgSGR = make(map[vt10x.Color]string)
	}

	target := session + ":" + tmux.SafeWindowName(window)
	cmd := exec.Command("tmux", "attach-session", "-t", target)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(et.rows),
		Cols: uint16(et.cols),
	})
	if err != nil {
		return err
	}

	et.cmd = cmd
	et.ptmx = ptmx
	return nil
}

// WriteInput sends raw bytes (keyboard input) to the PTY.
func (et *EmbeddedTerminal) WriteInput(data []byte) {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.ptmx != nil && !et.closed {
		et.ptmx.Write(data)
	}
}

// Resize changes the terminal dimensions and signals the PTY.
func (et *EmbeddedTerminal) Resize(cols, rows int) {
	et.mu.Lock()
	defer et.mu.Unlock()
	if cols == et.cols && rows == et.rows {
		return
	}
	et.cols = cols
	et.rows = rows
	if et.vt != nil {
		et.vt.Resize(cols, rows)
	}
	if et.ptmx != nil {
		pty.Setsize(et.ptmx, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
	}
}

// Render converts the virtual terminal screen to an ANSI-styled string.
// Includes cursor rendering as a reverse-video block at the cursor position.
func (et *EmbeddedTerminal) Render(width, height int) string {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.vt == nil {
		return ""
	}

	var sb strings.Builder
	cols, rows := et.vt.Size()
	if width < cols {
		cols = width
	}
	if height < rows {
		rows = height
	}

	cur := et.vt.Cursor()
	showCursor := et.vt.CursorVisible()

	for row := 0; row < rows; row++ {
		if row > 0 {
			sb.WriteByte('\n')
		}
		var prevFG, prevBG vt10x.Color
		var prevMode int16
		prevIsCursor := false
		needReset := false

		for col := 0; col < cols; col++ {
			g := et.vt.Cell(col, row)
			isCursor := showCursor && col == cur.X && row == cur.Y

			// Emit new SGR when style or cursor state changes
			if col == 0 || g.FG != prevFG || g.BG != prevBG || g.Mode != prevMode || isCursor != prevIsCursor {
				if needReset {
					sb.WriteString("\x1b[0m")
				}
				if isCursor {
					sb.WriteString("\x1b[7m")
					if sgr := et.glyphSGR(g); sgr != "" {
						sb.WriteString(sgr)
					}
					needReset = true
				} else {
					if sgr := et.glyphSGR(g); sgr != "" {
						sb.WriteString(sgr)
						needReset = true
					} else {
						needReset = false
					}
				}
				prevFG = g.FG
				prevBG = g.BG
				prevMode = g.Mode
				prevIsCursor = isCursor
			}

			ch := g.Char
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		if needReset {
			sb.WriteString("\x1b[0m")
		}
	}
	return sb.String()
}

// Close tears down the PTY and process.
func (et *EmbeddedTerminal) Close() {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.closed {
		return
	}
	et.closed = true
	if et.ptmx != nil {
		et.ptmx.Close()
		et.ptmx = nil
	}
	if et.cmd != nil && et.cmd.Process != nil {
		et.cmd.Process.Kill()
		et.cmd.Wait()
	}
}

func (et *EmbeddedTerminal) IsClosed() bool {
	et.mu.Lock()
	defer et.mu.Unlock()
	return et.closed
}

// readPTYCmd blocks until PTY output is available, feeds it to the VT emulator,
// and returns a message to trigger re-render.
func readPTYCmd(et *EmbeddedTerminal) tea.Cmd {
	return func() tea.Msg {
		et.mu.Lock()
		ptmx := et.ptmx
		closed := et.closed
		et.mu.Unlock()
		if ptmx == nil || closed {
			return ptyEOFMsg{}
		}

		buf := make([]byte, 32*1024)
		n, err := ptmx.Read(buf)
		if n > 0 {
			et.mu.Lock()
			et.vt.Write(buf[:n])
			et.mu.Unlock()
		}
		if err != nil {
			return ptyEOFMsg{err: err}
		}
		return ptyReadMsg{}
	}
}

// ensureSessionForTerminalCmd ensures a tmux session exists, then signals readiness.
func ensureSessionForTerminalCmd(env config.Environment) tea.Cmd {
	return func() tea.Msg {
		if err := tmux.CheckTmuxExists(); err != nil {
			return terminalSessionReadyMsg{err: err}
		}
		if err := tmux.EnsureSession(env); err != nil {
			return terminalSessionReadyMsg{err: err}
		}
		return terminalSessionReadyMsg{}
	}
}

// enterTerminalMode enters interactive terminal mode for the selected window.
func (m Model) enterTerminalMode() (tea.Model, tea.Cmd) {
	env, ok := m.currentEnv()
	if !ok {
		m.status = "No environment selected."
		return m, nil
	}
	session := tmux.SessionName(env.Name)
	if _, live := m.sessions[session]; !live {
		m.status = "Starting session..."
		return m, ensureSessionForTerminalCmd(env)
	}
	windows := m.currentWindowNames()
	if len(windows) == 0 || m.selectedWindow >= len(windows) {
		m.status = "No window available."
		return m, nil
	}
	window := windows[m.selectedWindow]

	// Compute terminal dimensions
	_, rightWidth := splitPaneWidths(m.width - 1)
	contentWidth := paneContentWidth(rightWidth)
	bodyHeight := m.height - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	previewHeight := bodyHeight - 4 // title + tabs + blank + margin
	if previewHeight < 1 {
		previewHeight = 1
	}

	et := newEmbeddedTerminal(contentWidth, previewHeight)
	if err := et.Attach(session, window); err != nil {
		m.status = "Terminal attach failed: " + err.Error()
		return m, nil
	}

	m.embeddedTerm = et
	m.terminalMode = true
	m.status = "Terminal mode — Ctrl+q to exit"
	return m, readPTYCmd(et)
}

// updateTerminalMode handles key events when in interactive terminal mode.
func (m Model) updateTerminalMode(key string) (tea.Model, tea.Cmd) {
	if key == "ctrl+q" {
		m.terminalMode = false
		if m.embeddedTerm != nil {
			m.embeddedTerm.Close()
			m.embeddedTerm = nil
		}
		m.status = focusedPaneStatus(m.focusPane)
		return m, nil
	}

	if m.embeddedTerm != nil {
		data := keyToBytes(key)
		if len(data) > 0 {
			m.embeddedTerm.WriteInput(data)
		}
	}
	return m, nil
}

// glyphSGR produces an ANSI SGR escape sequence for a vt10x glyph's style.
func (et *EmbeddedTerminal) glyphSGR(g vt10x.Glyph) string {
	var params []string

	if g.Mode&vtAttrBold != 0 {
		params = append(params, "1")
	}
	if g.Mode&vtAttrItalic != 0 {
		params = append(params, "3")
	}
	if g.Mode&vtAttrUnderline != 0 {
		params = append(params, "4")
	}
	if g.Mode&vtAttrBlink != 0 {
		params = append(params, "5")
	}
	if g.Mode&vtAttrReverse != 0 {
		params = append(params, "7")
	}

	if fg := et.cachedColorSGR(g.FG, false); fg != "" {
		params = append(params, fg)
	}
	if bg := et.cachedColorSGR(g.BG, true); bg != "" {
		params = append(params, bg)
	}

	if len(params) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(params, ";") + "m"
}

// cachedColorSGR returns the SGR fragment for a color, memoized per terminal.
func (et *EmbeddedTerminal) cachedColorSGR(c vt10x.Color, bg bool) string {
	cache := et.fgSGR
	if bg {
		cache = et.bgSGR
	}
	if cache != nil {
		if s, ok := cache[c]; ok {
			return s
		}
	}
	s := colorSGR(c, bg)
	if cache != nil {
		if len(cache) >= sgrCacheCap {
			if bg {
				et.bgSGR = make(map[vt10x.Color]string)
				cache = et.bgSGR
			} else {
				et.fgSGR = make(map[vt10x.Color]string)
				cache = et.fgSGR
			}
		}
		cache[c] = s
	}
	return s
}

// colorSGR converts a vt10x Color to SGR parameter string.
func colorSGR(c vt10x.Color, bg bool) string {
	if c == vtDefaultFG || c == vtDefaultBG {
		return ""
	}
	base := 30
	if bg {
		base = 40
	}

	if c.ANSI() {
		idx := int(c)
		if idx < 8 {
			return fmt.Sprintf("%d", base+idx)
		}
		// Bright colors (8-15)
		return fmt.Sprintf("%d", base+60+idx-8)
	}

	// 256-color or RGB
	n := uint32(c)
	if n < 256 {
		if bg {
			return fmt.Sprintf("48;5;%d", n)
		}
		return fmt.Sprintf("38;5;%d", n)
	}

	// RGB: encoded as r<<16 | g<<8 | b
	r := (n >> 16) & 0xff
	g := (n >> 8) & 0xff
	b := n & 0xff
	if bg {
		return fmt.Sprintf("48;2;%d;%d;%d", r, g, b)
	}
	return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
}

// keyToBytes converts a bubbletea key name to raw terminal escape bytes.
func keyToBytes(key string) []byte {
	switch key {
	case "enter":
		return []byte{'\r'}
	case "tab":
		return []byte{'\t'}
	case "shift+tab":
		return []byte{0x1b, '[', 'Z'}
	case "backspace":
		return []byte{0x7f}
	case "delete":
		return []byte{0x1b, '[', '3', '~'}
	case "insert":
		return []byte{0x1b, '[', '2', '~'}
	case "up":
		return []byte{0x1b, '[', 'A'}
	case "down":
		return []byte{0x1b, '[', 'B'}
	case "right":
		return []byte{0x1b, '[', 'C'}
	case "left":
		return []byte{0x1b, '[', 'D'}
	case "home":
		return []byte{0x1b, '[', 'H'}
	case "end":
		return []byte{0x1b, '[', 'F'}
	case "pgup":
		return []byte{0x1b, '[', '5', '~'}
	case "pgdown":
		return []byte{0x1b, '[', '6', '~'}
	case "esc":
		return []byte{0x1b}
	case "space":
		return []byte{' '}
	case "f1":
		return []byte{0x1b, 'O', 'P'}
	case "f2":
		return []byte{0x1b, 'O', 'Q'}
	case "f3":
		return []byte{0x1b, 'O', 'R'}
	case "f4":
		return []byte{0x1b, 'O', 'S'}
	case "f5":
		return []byte{0x1b, '[', '1', '5', '~'}
	case "f6":
		return []byte{0x1b, '[', '1', '7', '~'}
	case "f7":
		return []byte{0x1b, '[', '1', '8', '~'}
	case "f8":
		return []byte{0x1b, '[', '1', '9', '~'}
	case "f9":
		return []byte{0x1b, '[', '2', '0', '~'}
	case "f10":
		return []byte{0x1b, '[', '2', '1', '~'}
	case "f11":
		return []byte{0x1b, '[', '2', '3', '~'}
	case "f12":
		return []byte{0x1b, '[', '2', '4', '~'}
	default:
		// Ctrl combinations: ctrl+a → 0x01, ctrl+b → 0x02, ..., ctrl+z → 0x1a
		if ch, ok := strings.CutPrefix(key, "ctrl+"); ok && len(ch) == 1 {
			c := ch[0]
			if c >= 'a' && c <= 'z' {
				return []byte{c - 'a' + 1}
			}
			switch c {
			case '[':
				return []byte{0x1b}
			case '\\':
				return []byte{0x1c}
			case ']':
				return []byte{0x1d}
			case '^':
				return []byte{0x1e}
			case '_':
				return []byte{0x1f}
			}
			return nil
		}
		// Alt combinations: send ESC prefix
		if ch, ok := strings.CutPrefix(key, "alt+"); ok {
			return append([]byte{0x1b}, []byte(ch)...)
		}
		// Single printable character or unicode rune
		if len(key) >= 1 {
			return []byte(key)
		}
		return nil
	}
}
