# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build ./cmd/ide    # Build binary
go run ./cmd/ide      # Run without building
go fmt ./...          # Format code
go vet ./...          # Static analysis
go test ./...         # Run tests (none yet at MVP stage)
```

## Architecture

This is a TUI application using [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Model-Update-View pattern) that manages tmux-based development environments.

**Layer overview:**
- `cmd/ide/main.go` — Entry point; creates `tea.Program` with `ui.NewModel()`
- `internal/ui/model.go` — All TUI state, keyboard handling, and rendering (~2900 lines)
- `internal/config/config.go` — JSON config persistence at `~/.config/ide/environments.json`
- `internal/tmux/tmux.go` — Wrapper around tmux CLI commands

**Data flow:**
1. `Init()` fires `loadConfigCmd()` and `loadSessionsCmd()` concurrently
2. Async results arrive as typed messages (`configLoadedMsg`, `sessionsLoadedMsg`, etc.)
3. `Update()` handles messages and keyboard input, returns new model + optional commands
4. `View()` renders three panes: environments list (left-top), templates list (left-bottom), details (right)

**Key data structures:**
- `Environment` — name, root path, db connection string, list of `WindowTemplate`
- `WindowTemplate` — name, command to run, working directory
- `Template` — reusable named set of windows
- Tmux sessions are named `ide-<envname>` (e.g. "prod" → "ide-prod")

**Config file schema** (`~/.config/ide/environments.json`):
```json
{
  "environments": [{"name": "", "root": "", "db_connection": "", "windows": [...]}],
  "templates": [{"name": "", "windows": [...]}],
  "theme": "Midnight"
}
```

## Key patterns in model.go

- All async operations (tmux calls, config I/O) are issued as Bubble Tea `tea.Cmd` functions returning typed messages
- Focus is tracked via `focusPane` (0=environments, 1=templates, 2=details)
- Modal overlays (create form, theme picker, shortcuts help) are rendered on top of the base layout using lipgloss
- 20 built-in themes defined inline; active theme persisted to config
