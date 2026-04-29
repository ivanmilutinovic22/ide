# ide

A terminal UI for managing **tmux**-based development environments.

`ide` is a launcher and dashboard for the projects you work on every day.
Define each project once — its root directory, the windows you want open,
the commands they run — and `ide` handles spinning up the right `tmux`
session, attaching to it, and showing you what's running across every
project at a glance.

> **Note:** `ide` is a frontend for `tmux`, not a replacement. `tmux` 3.2+
> must be installed and on your `PATH` — without it, `ide` will not run.

```
┌─ Sessions ──────────────┬─ Windows ─────────────────────────────────┐
│ ● api-gateway      [↑]  │ ▸ editor   nvim                            │
│ ● web              [↑]  │   server   air ⠋                           │
│ ○ data-pipeline    [ ]  │   db       psql                            │
│ ○ infra            [ ]  │   logs     tail -f                         │
├─ Templates ─────────────┼─ Details ─────────────────────────────────┤
│ ▸ go-service            │ Root:    ~/code/api-gateway                │
│   next-app              │ Session: ide-api-gateway (attached)        │
│   data-notebook         │ DB:      postgres://...                    │
└─────────────────────────┴────────────────────────────────────────────┘
 [s]essions  [w]indows  [t]emplates   ?: help   q: quit
```

---

## Features

- **One keystroke to attach** — pick a project, hit `enter`, you're in `tmux`.
- **Reproducible window layouts** — each environment lists its windows
  (`editor`, `logs`, `server`, …), each with its own startup command and
  working directory. Re-create the layout any time with `r r`.
- **Live status across all projects** — see at a glance which sessions are
  running, what command is in the foreground of each window, and whether an
  AI agent (Claude Code, etc.) is busy or idle in any pane.
- **Templates** — save a window layout once and reuse it when creating new
  environments (`go-service`, `next-app`, …).
- **Fuzzy search** — `Ctrl+P` opens a search across every window in every
  environment; `enter` jumps you straight there.
- **Embedded preview** — the right pane shows a live capture of the
  selected window without leaving `ide`.
- **In-tmux popup** — once attached, `prefix + a` opens the same fuzzy
  search inside `tmux` via `display-popup`.
- **19 built-in themes**, switchable with `Ctrl+T`.

---

## Requirements

- **`tmux`** — version 3.2 or newer (needed for `display-popup`). This
  is a hard requirement; `ide` shells out to `tmux` for everything.
- **Go 1.24+** (only for building from source).
- A 256-color terminal. Most modern terminals work out of the box.

---

## Platform support

| Platform | Status |
|---|---|
| Linux (Arch) | Tested |
| macOS | Tested |
| Other Linux distros (Debian, Ubuntu, Fedora, …) | Likely works, untested |
| Windows | Not supported — `tmux` does not run natively on Windows; WSL2 may work but is untested. |

---

## Install

### From source

```bash
git clone https://github.com/<your-user>/ide.git
cd ide
go build -o ide .
mv ide ~/.local/bin/   # or anywhere on your PATH
```

`make build` produces the same binary at `./build/ide`.

### Verifying tmux

```bash
tmux -V    # should print 3.2 or higher
```

---

## Quick start

1. Run `ide` in your terminal. On first launch it creates an empty config
   at `~/.config/ide/environments.json` (or
   `$XDG_CONFIG_HOME/ide/environments.json`) and seeds it with a few
   built-in templates.
2. Press **`a`** to create your first environment. Give it a name, point
   it at a project root, and pick a template (or skip the template to
   start with a single shell window).
3. Press **`enter`** on the new environment to attach. `ide` creates the
   `tmux` session, opens the windows you defined, and drops you in.
4. Detach the way you always do (`Ctrl-b d` by default) — your session
   keeps running. Re-launch `ide` and you'll see it in the **Sessions**
   pane with an **`[↑]`** marker.

---

## Concepts

### Environment
A project. Has a name, a root directory, optionally a database
connection string, and a list of **windows**.

### Window
One `tmux` window inside an environment. Has a name, an optional
startup command, and an optional working directory (relative to the
environment root, or absolute).

### Template
A reusable, named list of windows. When you create a new environment
you can pick a template to seed its window list. Modifying the
environment afterwards does not affect the template.

### Session
A live `tmux` session created from an environment. Sessions are named
`ide-<env-name>` (e.g. environment `web` → session `ide-web`). `ide`
shows whether a session is running, what windows it has, and the
foreground command in each window.

---

## Configuration

The config file is plain JSON. You can edit it directly or use the UI.

**Path:** `~/.config/ide/environments.json` (Linux/macOS — follows
`os.UserConfigDir`).

```json
{
  "environments": [
    {
      "name": "api-gateway",
      "root": "/Users/me/code/api-gateway",
      "db_connection": "postgres://localhost/api_dev",
      "windows": [
        { "name": "editor",  "cmd": "nvim ." },
        { "name": "server",  "cmd": "air",   "cwd": "cmd/server" },
        { "name": "db",      "cmd": "psql $DATABASE_URL" },
        { "name": "logs",    "cmd": "tail -f logs/dev.log" }
      ]
    }
  ],
  "templates": [
    {
      "name": "go-service",
      "windows": [
        { "name": "editor", "cmd": "nvim ." },
        { "name": "server", "cmd": "air" },
        { "name": "test",   "cmd": "watchexec -e go go test ./..." }
      ]
    }
  ],
  "theme": "Midnight"
}
```

### Fields

| Field | Notes |
|---|---|
| `environments[].name` | Free-form. Sanitized for `tmux` (lowercased, spaces → `-`). |
| `environments[].root` | Absolute path. Used as the working directory for windows that don't override it. |
| `environments[].db_connection` | Optional. Used by the legacy default `database` window — be aware it's surfaced to the UI; treat its contents as you would any other line in this file. |
| `environments[].windows[].name` | Window name. Spaces are replaced with `-`. |
| `environments[].windows[].cmd` | Optional startup command. Runs inside `$SHELL -lc`, then drops you back into an interactive shell. |
| `environments[].windows[].cwd` | Optional. Relative paths are joined with the environment root. |
| `templates[]` | Same shape as `environments[].windows`, plus a `name`. |
| `theme` | One of the 19 built-in themes (see `Ctrl+T`). |

> **Heads up:** the config file may contain credentials (e.g. a Postgres
> URL with a password). It is created with default permissions; if that's
> a concern on a shared machine, run `chmod 600 ~/.config/ide/environments.json`.

---

## Keyboard reference

Press **`?`** at any time for the in-app shortcuts overlay. Highlights:

### Global

| Keys | Action |
|---|---|
| `1` / `2` / `3` | Focus Sessions / Windows / Templates |
| `Tab` | Cycle panels |
| `Ctrl+P` | Fuzzy search across all windows |
| `Ctrl+T` | Theme picker |
| `n` / `N` | Jump to next / previous AI window |
| `r` | Refresh sessions |
| `?` | Toggle shortcuts overlay |
| `q` / `Ctrl+C` | Quit |

### Sessions pane

| Keys | Action |
|---|---|
| `j` / `k` | Move selection |
| `Enter` | Attach to session (creates it if needed) |
| `a` | Create environment |
| `e` | Edit selected environment |
| `r r` | Restart session (kill + recreate) |
| `T` | Save current windows as a template |
| `d d` | Delete environment (config only) |
| `x x` | Kill `tmux` session |

### Windows pane

| Keys | Action |
|---|---|
| `h` / `l` | Move selection |
| `Enter` | Open the window in an embedded terminal |
| `H` / `L` | Reorder windows |
| `Ctrl+Q` | Exit embedded terminal |

### Templates pane

| Keys | Action |
|---|---|
| `j` / `k` | Move selection |
| `a` | New template |
| `e` / `Enter` | Edit template |
| `d d` | Delete template |

### Inside `tmux`

After `ide` creates a session it binds `prefix + a` to a popup that
opens the same fuzzy search inside the running `tmux` — no need to
detach to switch projects.

---

## How sessions are created

When you attach to an environment, `ide` runs roughly:

```
tmux new-session -d -s ide-<env> -n <first-window> -c <root> '<cmd>; exec $SHELL -i'
tmux new-window  -t ide-<env>    -n <next-window>  -c <cwd>  '<cmd>; exec $SHELL -i'
…
tmux attach-session -t ide-<env>
```

If the session already exists, `ide` skips creation and attaches. The
`exec $SHELL -i` tail means a window's startup command runs once, then
leaves you with an interactive shell — exit the command and you're back
at a prompt instead of the window closing.

---

## Troubleshooting

**`tmux is not installed or not in PATH`**
Install `tmux` 3.2+ from your package manager (`brew install tmux`,
`apt install tmux`, …).

**The session attaches but a window's command didn't run**
Check `$SHELL`. Startup commands are wrapped in `$SHELL -lc`; if your
shell is misconfigured the wrapper exits early. Inspect
`/tmp/ide-debug.log` for the exact `tmux` invocation.

**Restart didn't pick up my config change**
`r r` kills and recreates the session, but if the session was already
attached elsewhere, detach first or use `x x` to kill it before
attaching anew.

**Fuzzy search shows stale windows**
Press `r` to refresh. `ide` snapshots `tmux` on launch and on
attach/restart; it doesn't poll continuously.

**Where are debug logs?**
`/tmp/ide-debug.log`.

---

## Development

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

See [`CLAUDE.md`](./CLAUDE.md) for the architectural overview.

---

## License

Licensed under the [Apache License, Version 2.0](./LICENSE).
Copyright 2026 Vladimir Filipovic, Ivan Milutinovic.
