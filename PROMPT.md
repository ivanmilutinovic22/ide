# ide — Ralph Wiggum Loop Spec

This file is designed for the autonomous agent loop:

```
while :; do cat PROMPT.md | claude-code; done
```

Read `.ralph/prd.json` at the start of each run to find stories with `status: "open"`. Implement those stories, mark them `"done"` in `.ralph/prd.json`, run `go build ./cmd/ide && go vet ./...` to verify, then exit.

---

## Project context

A TUI application (`internal/ui/model.go`, ~3300 lines) using Bubble Tea (Model-Update-View) that manages tmux-based development environments.

Key files:

- `internal/config/config.go` — data structures, JSON persistence
- `internal/tmux/tmux.go` — tmux CLI wrappers
- `internal/ui/model.go` — all TUI logic
- `~/.config/ide/environments.json` — user config

Build: `go build ./cmd/ide`
Vet: `go vet ./...`

---

## Implemented features (all `"done"` in prd.json)

### Feature 1 — Window Type: Normal vs Agent

- `WindowTemplate.Type string` added to `internal/config/config.go`
- `parseWindowEntry` supports `name=cmd|cwd|agent` and `name=cmd||agent` syntax
- `formatWindowSpec` emits `|agent` suffix when `Type == "agent"`
- `windowTypeFor(env, windowName)` helper in model.go

### Feature 2 — Background 1s Poll Loop + Per-Window Runtime State + Status Badges

- `tickCmd()` using `tea.Tick(time.Second, ...)`
- `captureAllWindowsCmd(session, windows)` captures all windows concurrently
- `classifyWindowState(snap, prev)` state machine: "running" / "thinking" / "waiting"
- `windowRuntime map[string]windowRuntimeState` on Model
- Agent windows show `[running]`/`[thinking]`/`[waiting]` badges in window tabs
- `State: <status>` shown in info section for selected agent window

### Feature 4 — Import Current Tmux Session (I key)

- `tmux.CurrentTmuxSession()` reads `$TMUX` env + `tmux display-message`
- `importCurrentSessionCmd(sessionName)` creates env entry from live session windows
- Press `I` in Sessions panel to import (no-op if not inside tmux)

### Feature 6 — Add Window to Running Session

- `tmux.AddWindow(session, WindowTemplate)` in tmux.go
- Press `a` in Windows panel when session is live → opens Add Window modal
- `addWindowToSessionCmd` calls `tmux.AddWindow` then saves to config

---

## Future stories (add to prd.json and implement in next loop)

- Responsive layout (handle very narrow/wide terminals)
- Preview ANSI color rendering (strip escape codes or render properly)
- Premade template windows library
- Fix window rearrange bug when session has extra windows not in config
- Multi-session support in tick loop (poll all live sessions, not just selected env)
