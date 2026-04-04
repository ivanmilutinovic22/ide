# Embedded Interactive Terminal Pane

## Overview

The Windows pane (right side of the 3-pane layout) was reworked from a **read-only preview** that polled `tmux capture-pane` every 500ms into a **fully interactive embedded terminal** powered by a real PTY and a virtual terminal emulator. Keystrokes are written directly to the PTY as raw bytes; terminal output is read from the PTY as it arrives (zero-polling, event-driven), parsed through a VT100 emulator, and rendered back to ANSI for display inside the Bubble Tea TUI.

---

## Problem with the previous approach

The old implementation used two tmux CLI commands for I/O:

- **Output**: `tmux capture-pane -p -e -t session:window` polled on a 500ms timer, returning a snapshot of the pane content with ANSI escape codes.
- **Input**: `tmux send-keys -t session:window <key>` spawned a subprocess for every single keystroke.

Both paths go through `exec.Command` → fork/exec → tmux IPC → tmux server. Each round-trip adds 5-20ms of latency. At 500ms polling the output is visibly stale; even at 50ms polling (~20fps) there is perceptible lag on every keystroke because the send-keys subprocess must finish, the tmux server must process it, then the next capture must run.

---

## Architecture of the new approach

```
  Bubble Tea TUI (bubbletea)
  ┌─────────────────────────────────────────────────┐
  │  Model.Update()                                 │
  │    ├─ KeyMsg → keyToBytes() → ptmx.Write()      │  ← direct PTY write
  │    ├─ ptyReadMsg → readPTYCmd(et)                │  ← chain next read
  │    └─ ptyEOFMsg → cleanup                        │
  │                                                 │
  │  Model.View()                                   │
  │    └─ renderTerminalPane()                       │
  │         └─ embeddedTerm.Render(w, h)             │  ← read vt10x screen
  └─────────────────┬───────────────────────────────┘
                    │
                    │ PTY (pseudo-terminal)
                    │ created by creack/pty
                    │
  ┌─────────────────▼───────────────────────────────┐
  │  tmux attach-session -t ide-myenv:editor         │
  │  (runs as a child process of the PTY)            │
  │                                                 │
  │  tmux server manages the actual session windows  │
  │  and renders to our PTY as if it were a terminal │
  └─────────────────────────────────────────────────┘
```

Three components work together:

### 1. PTY (`creack/pty`)

A pseudo-terminal is a kernel-level pair of file descriptors (master/slave) that emulates a hardware terminal. `creack/pty` wraps the POSIX `openpty`/`forkpty` calls for Go.

We call `pty.StartWithSize(cmd, &pty.Winsize{...})` which:
1. Creates a PTY pair
2. Forks the `tmux attach-session` process with the slave end as its stdin/stdout/stderr
3. Returns the master end (`*os.File`) to us

From our side, `ptmx.Read()` gets the terminal output (escape sequences and all) and `ptmx.Write()` sends keyboard input. This is the same interface a real terminal emulator like iTerm2 or Alacritty uses.

### 2. Virtual Terminal Emulator (`hinshun/vt10x`)

The raw bytes from the PTY are VT100/xterm escape sequences (cursor movement, color changes, screen clearing, scrolling, alternate screen buffer, etc.). We cannot display these raw bytes in our pane because Bubble Tea owns the real terminal. Instead, we feed them into `vt10x`, a Go implementation of a VT100/xterm terminal emulator that maintains a virtual screen buffer in memory.

`vt10x.Terminal` implements `io.Writer`. When we write PTY output to it, it parses all escape sequences and updates its internal grid of `Glyph` cells. Each cell has:

```go
type Glyph struct {
    Char rune      // the character
    Mode int16     // attribute bitmask (bold, italic, underline, reverse, blink)
    FG   Color     // foreground color (default, ANSI 0-15, xterm 16-255, or RGB)
    BG   Color     // background color
}
```

We can then read this grid cell-by-cell via `vt.Cell(col, row)` and convert it back to ANSI SGR escape sequences for rendering.

### 3. Bubble Tea command chain

Bubble Tea's architecture is `Init → Update → View → Update → View → ...`. All I/O must go through `tea.Cmd` functions that return `tea.Msg` values. We cannot spawn background goroutines that mutate state directly.

The read loop works via command chaining:

```
enterTerminalMode()
  → issues readPTYCmd(et)
    → blocks on ptmx.Read(buf)
    → feeds data to et.vt.Write(buf[:n])
    → returns ptyReadMsg{}

Update receives ptyReadMsg
  → Bubble Tea calls View() → renderTerminalPane() → et.Render()
  → issues readPTYCmd(et) again
    → blocks on next ptmx.Read()
    → ...
```

Each `ptyReadMsg` causes a re-render (because `Update` returned a new model) and immediately starts waiting for the next chunk. There is no timer, no polling interval. Output appears as fast as the PTY produces it and Bubble Tea can render it.

---

## Detailed file-by-file changes

### `internal/ui/terminal.go` (new file)

This file contains everything related to the embedded terminal: the `EmbeddedTerminal` struct, PTY lifecycle management, VT rendering, key mapping, and Bubble Tea integration commands.

#### `EmbeddedTerminal` struct

```go
type EmbeddedTerminal struct {
    mu      sync.Mutex       // protects all fields
    vt      vt10x.Terminal   // virtual screen buffer
    ptmx    *os.File         // PTY master file descriptor
    cmd     *exec.Cmd        // tmux attach process
    cols    int
    rows    int
    session string
    window  string
    closed  bool
}
```

All access is guarded by `sync.Mutex` because the PTY read command runs in a separate goroutine (Bubble Tea runs `tea.Cmd` functions concurrently).

#### `Attach(session, window string) error`

1. Calls `Close()` to tear down any previous PTY
2. Creates a new `vt10x.Terminal` with `vt10x.New(vt10x.WithSize(cols, rows))`
3. Builds `exec.Command("tmux", "attach-session", "-t", "session:window")` with `TERM=xterm-256color`
4. Calls `pty.StartWithSize(cmd, &pty.Winsize{...})` to fork the process in a PTY
5. Stores the PTY master fd and command handle

#### `WriteInput(data []byte)`

Writes raw bytes directly to `ptmx`. This is how keyboard input reaches tmux. No subprocess, no IPC overhead. A single `write()` syscall.

#### `Resize(cols, rows int)`

Updates both the VT emulator (`vt.Resize(cols, rows)`) and the PTY (`pty.Setsize(ptmx, &Winsize{...})`). The PTY resize sends a `SIGWINCH` signal to tmux, which redraws its content for the new dimensions.

#### `Render(width, height int) string`

Iterates over the VT screen grid cell by cell. For each cell it:

1. Checks if the style (FG, BG, attributes) changed from the previous cell
2. If so, emits `\x1b[0m` (reset) followed by a new SGR sequence via `glyphSGR()`
3. Writes the character (or space if the cell is empty, i.e., `Char == 0`)
4. At end of each row, emits `\x1b[0m` to reset

The SGR sequence is built from the `Glyph.Mode` bitmask (bold=bit 2, italic=bit 4, underline=bit 1, blink=bit 5, reverse=bit 0) and the FG/BG colors.

#### `glyphSGR(g vt10x.Glyph) string`

Converts a glyph's style to an ANSI SGR escape:

- Checks each attribute bit and adds the corresponding SGR parameter (1=bold, 3=italic, 4=underline, 5=blink, 7=reverse)
- Calls `colorSGR()` for FG and BG

#### `colorSGR(c vt10x.Color, bg bool) string`

Handles the vt10x color model:

- **Default colors** (`1<<24`, `1<<24+1`): returns empty string (use terminal default)
- **ANSI 0-7**: returns SGR 30-37 (FG) or 40-47 (BG)
- **ANSI 8-15** (bright): returns SGR 90-97 or 100-107
- **256-color palette** (values < 256): returns `38;5;N` or `48;5;N`
- **RGB** (values >= 256, encoded as `r<<16 | g<<8 | b`): returns `38;2;r;g;b` or `48;2;r;g;b`

#### `readPTYCmd(et *EmbeddedTerminal) tea.Cmd`

Returns a `tea.Cmd` that:

1. Grabs the `ptmx` file descriptor under the lock
2. Calls `ptmx.Read(buf)` which **blocks** until data is available (this is the key - no polling)
3. Locks the mutex and writes the data to `et.vt.Write(buf[:n])`
4. Returns `ptyReadMsg{}` on success or `ptyEOFMsg{err}` on error/EOF

#### `keyToBytes(key string) []byte`

Maps Bubble Tea's key names to raw terminal escape sequences:

| Bubble Tea key | Bytes sent to PTY |
|---|---|
| `"enter"` | `\r` (0x0D) |
| `"tab"` | `\t` (0x09) |
| `"backspace"` | DEL (0x7F) |
| `"esc"` | ESC (0x1B) |
| `"up"` | `\x1b[A` |
| `"down"` | `\x1b[B` |
| `"left"` | `\x1b[C` |
| `"right"` | `\x1b[D` |
| `"home"` | `\x1b[H` |
| `"end"` | `\x1b[F` |
| `"pgup"` | `\x1b[5~` |
| `"pgdown"` | `\x1b[6~` |
| `"delete"` | `\x1b[3~` |
| `"f1"`-`"f12"` | Standard xterm F-key sequences |
| `"ctrl+a"`-`"ctrl+z"` | 0x01-0x1A (ASCII control codes) |
| `"alt+x"` | `\x1b` + the character (ESC prefix) |
| `"shift+tab"` | `\x1b[Z` |
| Single characters | The character's UTF-8 bytes |

#### `enterTerminalMode() (tea.Model, tea.Cmd)`

1. Gets the current environment and checks if its tmux session is live
2. If not live, fires `ensureSessionForTerminalCmd()` to create the session asynchronously, then returns (terminal mode is entered when `terminalSessionReadyMsg` arrives)
3. If live, computes the terminal dimensions from the pane layout:
   - `contentWidth = paneContentWidth(rightWidth)` (pane width minus padding)
   - `previewHeight = bodyHeight - 4` (minus title, tabs, blank separator, margin)
4. Creates `newEmbeddedTerminal(contentWidth, previewHeight)`
5. Calls `et.Attach(session, window)` to start the PTY
6. Sets `m.embeddedTerm = et` and `m.terminalMode = true`
7. Returns `readPTYCmd(et)` to start the read loop

#### `updateTerminalMode(key string) (tea.Model, tea.Cmd)`

- If `key == "ctrl+]"`: exits terminal mode, closes the embedded terminal, returns to navigation
- Otherwise: converts the key to bytes via `keyToBytes()` and writes to `et.WriteInput(data)`
- Returns no command (the read loop is already running independently)

---

### `internal/ui/model.go` (modified)

#### New fields on `Model`

```go
terminalMode bool              // true when in interactive terminal mode
embeddedTerm *EmbeddedTerminal // the live PTY + VT emulator, nil when not in terminal mode
```

#### `Update()` changes

**Window resize** (`tea.WindowSizeMsg`): When `terminalMode` is true and `embeddedTerm` is non-nil, computes the new terminal dimensions and calls `embeddedTerm.Resize(cols, rows)`. This sends SIGWINCH to tmux.

**Key dispatch** (`tea.KeyMsg`): Terminal mode check is placed **before** global shortcuts (`ctrl+t`, `ctrl+p`, `?`) so that those keys reach the embedded terminal instead of being intercepted. The check also excludes overlay modes (fuzzy search, theme picker, shortcuts help) so those overlays still work:

```go
if m.terminalMode && !m.showFuzzySearch && !m.showThemePicker && !m.showShortcuts {
    return m.updateTerminalMode(key)
}
```

**New message handlers**:

- `ptyReadMsg`: The VT emulator already has the new data (written in `readPTYCmd`). Just issue another `readPTYCmd(et)` to keep reading. Bubble Tea re-renders because `Update` returned.
- `ptyEOFMsg`: PTY closed (tmux detached or session killed). Sets `terminalMode = false`, closes the embedded terminal, reloads sessions list.
- `terminalSessionReadyMsg`: Session was just created via `ensureSessionForTerminalCmd`. Adds the session to `m.sessions` immediately (so `enterTerminalMode` finds it without waiting for async reload), populates `m.sessionWindows`, then calls `enterTerminalMode()`.

**Enter key in Windows pane**: Changed from `startAttachSelected()` (which did a full-screen `tea.ExecProcess` tmux attach) to `enterTerminalMode()`.

**Cleanup on exit**: When `q`/`ctrl+c` is pressed, `embeddedTerm.Close()` is called. When Tab or pane shortcuts switch focus, terminal mode is exited and the embedded terminal is closed.

#### `renderDetailsPane()` changes

Refactored into three helper functions:

- **`renderDetailsPane()`**: Entry point. If `terminalMode` is true, delegates to `renderTerminalPane()`. Otherwise renders the normal Windows view.
- **`renderTerminalPane()`**: Renders with minimal chrome (tabs + blank + VT output). Calls `embeddedTerm.Render(contentWidth, previewHeight)` to get the ANSI string from the VT screen. Pads remaining rows with the theme background.
- **`renderWindowTabs()`**: Extracted tab bar rendering (shared between normal and terminal views).
- **`renderPreviewRows()`**: Extracted the capture-pane preview rendering (used by the normal view when not in terminal mode).

#### Status bar and shortcuts

- Context hints show `ctrl+] exit terminal` and `keys → tmux` when in terminal mode
- Windows pane hints changed from `enter attach` to `enter terminal`
- Shortcuts list updated: `enter → enter terminal mode`, added `ctrl+] → exit terminal mode`

---

### `internal/tmux/tmux.go` (modified)

Exported `safeWindowName` → `SafeWindowName` so the `ui` package can use it to build tmux targets.

---

### `go.mod` (modified)

Added two new dependencies:

- `github.com/creack/pty v1.1.24` - POSIX pseudo-terminal handling for Go
- `github.com/hinshun/vt10x v0.0.0-20220301184237-5011da428d02` - VT100/xterm terminal emulator in Go

---

## Data flow: a keystroke round-trip

Here is exactly what happens when you press `l` in terminal mode, from keypress to screen update:

1. **Bubble Tea reads the key** from stdin and delivers `tea.KeyMsg{Type: tea.KeyRunes, Runes: ['l']}` to `Update()`
2. **`Update()`** sees `m.terminalMode == true`, calls `m.updateTerminalMode("l")`
3. **`updateTerminalMode()`** calls `keyToBytes("l")` which returns `[]byte{'l'}` (0x6C)
4. **`WriteInput()`** calls `ptmx.Write([]byte{'l'})` - a single `write(2)` syscall to the PTY master fd
5. **The kernel** delivers the byte to the PTY slave fd, which is tmux's stdin
6. **tmux** processes the keypress (sends it to the shell in the active pane, the shell echoes it, the shell may update the prompt, etc.)
7. **tmux** renders the updated screen to the PTY slave fd (its stdout), using ANSI escape sequences
8. **The kernel** makes the data available on the PTY master fd
9. **`readPTYCmd()`** was already blocked on `ptmx.Read()`. It wakes up, reads the bytes (e.g., `l` echo + maybe cursor movement sequences)
10. **Under the lock**, it calls `et.vt.Write(buf[:n])` which updates the VT screen buffer
11. **Returns `ptyReadMsg{}`** to Bubble Tea
12. **`Update()`** handles `ptyReadMsg`, issues another `readPTYCmd()` for the next read
13. **Because `Update` returned**, Bubble Tea calls `View()` which calls `renderTerminalPane()`
14. **`renderTerminalPane()`** calls `et.Render(width, height)` which reads the VT grid, converts each cell's attributes back to SGR codes, and builds an ANSI string
15. **Bubble Tea** diffs the output against the previous frame and writes the changes to the real terminal

Steps 4-15 happen with no subprocess spawning, no tmux CLI invocations, no polling. The latency is bounded by: one `write()` syscall + tmux server processing + one `read()` syscall + VT processing + Bubble Tea render. In practice this is sub-millisecond for the I/O and a few milliseconds for rendering.

---

## UX flow

1. Navigate to the **Windows** pane (press `2` or `Tab`)
2. Select a window with `h`/`l` (left/right)
3. Press **Enter** to enter terminal mode
   - If the session isn't running, it's created first (async), then terminal mode activates
   - The tmux pane is resized to match the display area
4. **Type normally** - all keys go to the embedded tmux session
   - You get the full tmux experience: tmux prefix keys work, vim works, shell completion works
5. Press **Ctrl+]** to exit terminal mode and return to navigation
6. The embedded terminal is closed (PTY killed, VT freed)
7. The preview reverts to the capture-pane based view at 500ms polling

Full-screen tmux attach is still available via **Enter** from the Sessions pane (left-top).
