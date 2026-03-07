# Conversation: tmux window preview & OSC 11 terminal background

## How does tmux preview open windows when pressing leader+w?

When you press `prefix + w` in tmux, it runs `choose-tree -Zw` (or `choose-window`), which opens tmux's built-in window chooser. The preview shows the actual content of each window/pane using tmux's internal rendering.

**The key is `capture-pane -e`**

The `CapturePane` function in `internal/tmux/tmux.go` uses:

```go
cmd := exec.Command("tmux", "capture-pane", "-p", "-e", "-t", target)
```

The `-e` flag tells tmux to include ANSI escape sequences in the captured output — that's what preserves the background colors and styling in the preview.

**What `leader + w` does internally:**

1. Runs `choose-tree -Zw` (or `choose-window`)
2. For each window, tmux calls `capture-pane -e` to snapshot the pane content with its full ANSI escape codes
3. It renders that captured content as a live preview panel on the right side
4. The preview is re-captured periodically as content changes

**Why colors work:** tmux tracks the exact cell-by-cell state of each pane internally (foreground color, background color, attributes per cell). When you do `capture-pane -e`, it serializes that internal cell grid back into ANSI escape sequences, which any terminal can re-render faithfully.

---

## Can we display the captured output another way, not through lipgloss?

**What lipgloss is:**

Lipgloss is a Go library for styling terminal output — think CSS for the terminal. It wraps strings in ANSI escape codes to add colors, borders, padding, alignment. When you nest lipgloss-styled blocks, it recalculates sizing and can strip/overwrite embedded ANSI codes from other sources (like `capture-pane -e` output), which breaks the colors.

**How to see the raw output right now:**

```bash
tmux capture-pane -p -e -t <session>:<window>
```

You'll see the raw ANSI-escaped content printed directly — colors and all.

**Displaying it in a Bubble Tea app without lipgloss:**

- **Option 1 — Embed raw ANSI directly in `View()` string:** Concatenate the captured output into the returned string without wrapping in `lipgloss.Render()`. The terminal interprets it natively. The downside is lipgloss can't measure its width/height for layout purposes.
- **Option 2 — Use `tea.Program` with `WithAltScreen` and write directly:** For a full-screen passthrough preview panel, some TUI apps suspend Bubble Tea (`p.ReleaseTerminal()`), write raw bytes to stdout, then restore (`p.RestoreTerminal()`).
- **Option 3 — Use `ansi` stripping for layout, raw for display:** Use `muesli/ansi` or `charmbracelet/x/ansi` to strip codes when measuring width, but keep the raw string when writing.

---

## How does the current code handle background color, and what heuristics are used?

**How the current code sidesteps lipgloss for preview content (`model.go:2197–2225`):**

1. Each line is measured with `ansi.StringWidth(line)` (which correctly ignores escape codes)
2. If the line has ANSI sequences → `injectBGIntoLine()` is called, which manually prepends a raw `\x1b[48;2;R;G;Bm` sequence and re-injects it after every reset `\x1b[0m`
3. If it's plain text → lipgloss IS used just for background fill of empty space

So content lines go into `previewRows []string` as raw strings, then `strings.Join(allRows, "\n")` — only the outer border/title box goes through lipgloss.

**The heuristic — `detectPreviewBG()` at `model.go:3155`:**

Since there's no API to ask "what is this terminal's background color", it scans all `\x1b[48;...m` (SGR background) sequences in the captured content and picks the **darkest one** by luminance. The assumption: the terminal's own background is always the darkest explicit color set by the app running inside.

This breaks when:
- The terminal has a **light theme** (the darkest color would be foreground text, not background)
- The app inside uses **no explicit background colors** — `detectPreviewBG` returns `""` and falls back to `theme.PaneBG`

---

## Can we query the real terminal background color via OSC 11?

Yes — via the **OSC 11** escape sequence. You send `\x1b]11;?\x1b\\` to the terminal and it responds with `\x1b]11;rgb:RRRR/GGGG/BBBB\x1b\\`.

Supported by most modern terminal emulators: kitty, alacritty, foot, xterm, iTerm2, etc.

Caveats:
- You're querying the terminal your app runs in (same physical terminal as tmux windows — everything renders through the same terminal)
- You have to read the response from stdin while in raw mode, with a timeout
- Bubble Tea controls stdin, so it must be done **before `tea.Program` starts**

---

## Implementation: OSC 11 query at startup

### New file: `internal/terminal/bg.go`

```go
package terminal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// QueryBackgroundColor sends an OSC 11 query to the terminal and returns the
// background color as "#rrggbb". Returns "" if the terminal doesn't respond or
// the response can't be parsed. Must be called before bubbletea starts.
func QueryBackgroundColor() string {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return ""
	}
	defer tty.Close()

	fd := tty.Fd()
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, oldState)

	fmt.Fprint(tty, "\x1b]11;?\x1b\\")

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := tty.Read(buf)
		if err != nil || n == 0 {
			done <- ""
			return
		}
		done <- string(buf[:n])
	}()

	var raw string
	select {
	case raw = <-done:
	case <-time.After(200 * time.Millisecond):
		return ""
	}

	return parseOSC11(raw)
}
```

`parseOSC11` parses `\x1b]11;rgb:RRRR/GGGG/BBBB\x1b\\`. Terminals send 16-bit per channel (4 hex digits), so it takes the high byte (first 2 digits) to get 8-bit `#rrggbb`.

### Changes to `internal/ui/model.go`

- Added `terminalBG string` field to `Model`
- Updated `NewModel(terminalBG string)` to accept the queried color
- Updated the fallback chain in the preview renderer:

```
captured content BG  →  OSC 11 terminal BG  →  theme.PaneBG
```

```go
previewBGColor := m.previewBG
if previewBGColor == "" {
    previewBGColor = m.terminalBG
}
if previewBGColor == "" {
    previewBGColor = theme.PaneBG
}
```

### Changes to `cmd/ide/main.go`

```go
// Query the terminal background color before bubbletea takes over stdin.
termBG := terminal.QueryBackgroundColor()
log.Printf("terminal background color: %q", termBG)

p := tea.NewProgram(ui.NewModel(termBG), tea.WithAltScreen())
```
