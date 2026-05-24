package ui

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"

	"ide/internal/config"
	"ide/internal/layout"
	"ide/internal/tmux"
)

// EmbeddedTerminal manages a PTY running tmux attach with a virtual terminal emulator.
type EmbeddedTerminal struct {
	mu      sync.Mutex
	vt      *vt.Emulator
	ptmx    *os.File
	cmd     *exec.Cmd
	cols    int
	rows    int
	session string
	window  string
	closed  bool
}

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
		cols: cols,
		rows: rows,
	}
}

// Attach starts a PTY running tmux attach-session for the given target.
func (et *EmbeddedTerminal) Attach(session, window string) error {
	et.Close()
	et.mu.Lock()
	defer et.mu.Unlock()

	et.vt = vt.NewEmulator(et.cols, et.rows)
	et.session = session
	et.window = window
	et.closed = false

	target := session + ":" + tmux.SafeWindowName(window)
	cmd := exec.Command("tmux", "attach-session", "-t", target)
	// Strip TMUX/TMUX_PANE so the embedded client doesn't see itself as
	// nested — tmux refuses to attach when $TMUX is set unless forced, which
	// otherwise leaves the PTY blank.
	env := os.Environ()
	filtered := env[:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=") {
			continue
		}
		filtered = append(filtered, kv)
	}
	cmd.Env = append(filtered, "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(et.rows),
		Cols: uint16(et.cols),
	})
	if err != nil {
		return err
	}

	et.cmd = cmd
	et.ptmx = ptmx

	// Pump emulator responses (DA1, DA2, DSR, etc.) back into the PTY input.
	// tmux blocks drawing until it receives a Primary Device Attributes reply,
	// so without this the pane stays blank forever.
	go pumpEmulatorReplies(et.vt, ptmx)

	return nil
}

// pumpEmulatorReplies reads bytes that the emulator writes in response to
// terminal queries (e.g. \x1b[c → DA1) and forwards them to the PTY so the
// attached process can read them as if from a real terminal.
func pumpEmulatorReplies(em *vt.Emulator, ptmx *os.File) {
	buf := make([]byte, 1024)
	for {
		n, err := em.Read(buf)
		if n > 0 {
			if _, werr := ptmx.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// WriteInput sends raw bytes (keyboard input) to the PTY. Loops over short
// writes; on hard error the terminal is marked closed so the next read sees
// an EOF and tears the session down rather than silently swallowing input.
func (et *EmbeddedTerminal) WriteInput(data []byte) {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.ptmx == nil || et.closed {
		return
	}
	for len(data) > 0 {
		n, err := et.ptmx.Write(data)
		if err != nil {
			log.Printf("[Terminal] WriteInput error: %v", err)
			return
		}
		data = data[n:]
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

// Render returns the terminal screen as an ANSI-styled string, clipped to
// width x height. Lines past the requested height are dropped from the bottom.
func (et *EmbeddedTerminal) Render(width, height int) string {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.vt == nil {
		return ""
	}
	rendered := et.vt.Render()
	if rendered == "" {
		return ""
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > width {
			lines[i] = ansi.Cut(line, 0, width)
		}
	}
	return strings.Join(lines, "\n")
}

// Close tears down the PTY and process. The kill+wait runs WITHOUT holding
// et.mu so readPTYCmd's post-read mutex re-acquisition can't deadlock
// against the wait — and so callers blocked in Render/Resize don't stall
// while we reap the tmux subprocess.
func (et *EmbeddedTerminal) Close() {
	et.mu.Lock()
	if et.closed {
		et.mu.Unlock()
		return
	}
	et.closed = true
	vtRef := et.vt
	ptmx := et.ptmx
	cmd := et.cmd
	et.ptmx = nil
	et.mu.Unlock()

	if vtRef != nil {
		// Unblocks pumpEmulatorReplies' em.Read so the goroutine exits.
		_ = vtRef.Close()
	}
	if ptmx != nil {
		ptmx.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
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
			// Recheck closed: Close() may have run while we were blocked in
			// Read(). Without this we'd write into a torn-down emulator.
			if !et.closed && et.vt != nil {
				et.vt.Write(buf[:n])
			}
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

	_, rightWidth := splitPaneWidths(m.width - 1)
	et := newEmbeddedTerminal(paneContentWidth(rightWidth), layout.TerminalPreviewHeight(m.height))
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
//
// Two ways to leave terminal mode:
//   - ctrl+q  — direct exit
//   - ctrl+b q — tmux-leader style: press prefix, then q. The first ctrl+b is
//     buffered (not forwarded yet); if the next key is q we exit, otherwise
//     we forward both keys so other prefix bindings (e.g. ctrl+b 1) still work.
func (m Model) updateTerminalMode(key string) (tea.Model, tea.Cmd) {
	if key == "ctrl+q" {
		m.leaderPending = false
		return m.exitTerminalMode(), nil
	}

	if m.leaderPending {
		m.leaderPending = false
		if key == "q" {
			return m.exitTerminalMode(), nil
		}
		// Not our binding — replay the buffered ctrl+b before this key.
		if m.embeddedTerm != nil {
			m.embeddedTerm.WriteInput(keyToBytes("ctrl+b"))
			if data := keyToBytes(key); len(data) > 0 {
				m.embeddedTerm.WriteInput(data)
			}
		}
		return m, nil
	}

	if key == "ctrl+b" {
		m.leaderPending = true
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

func (m Model) exitTerminalMode() Model {
	m.terminalMode = false
	if m.embeddedTerm != nil {
		m.embeddedTerm.Close()
		m.embeddedTerm = nil
	}
	m.status = focusedPaneStatus(m.focusPane)
	return m
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
		// Alt combinations: send ESC prefix in front of the underlying key's
		// bytes. Recurse so e.g. "alt+ctrl+x" becomes ESC + 0x18 instead of
		// ESC + the literal string "ctrl+x".
		if ch, ok := strings.CutPrefix(key, "alt+"); ok {
			inner := keyToBytes(ch)
			if len(inner) == 0 {
				return nil
			}
			return append([]byte{0x1b}, inner...)
		}
		// Single printable character or unicode rune. Bubbletea reports
		// multi-rune names like "shift+f13" for keys we don't translate
		// above — drop them rather than smuggle the literal name into the PTY.
		if r := []rune(key); len(r) == 1 {
			return []byte(string(r))
		}
		return nil
	}
}
