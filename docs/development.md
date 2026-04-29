# Development

```bash
make build    # build to ./build/ide
make run      # go run .
make fmt      # go fmt ./...
make vet      # go vet ./...
make test     # go test ./...
```

The codebase is laid out as:

```
main.go                  one-line entry point
cmd/run/                 argv dispatch
run/                     bubble-tea program runners (main + search popup)
internal/ui/             TUI: model, update, view, modals, themes
internal/config/         JSON config persistence
internal/tmux/           tmux CLI wrapper
internal/agentstatus/    AI-agent activity detection
internal/theme/          color palettes
internal/layout/         pane geometry math
```

See [`../CLAUDE.md`](../CLAUDE.md) for the architectural overview.
