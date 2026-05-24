---
name: manage-ide-sessions
description: Create, inspect, and modify `ide` environments, templates, and windows from the command line. Use when an AI agent needs to spin up a project layout for the user (e.g. add a new worktree, register a service, swap a window command) without driving the TUI.
---

# Manage `ide` sessions

`ide` exposes a non-interactive CLI that reads and writes the same
`~/.config/ide/environments.json` file the TUI uses. Every TUI action that
edits config has a CLI equivalent, which makes the tool scriptable and lets
an AI agent prepare a working environment for the user before they ever
launch the TUI.

After the agent edits config, the user picks the environment in `ide` and
hits `enter`; `tmux` builds the session from the layout the agent just
wrote.

## When to reach for this

- The user asks to "add a worktree" or "set up a new project in `ide`".
- The user wants a window's command, cwd, or name changed and you'd
  otherwise tell them to open the TUI.
- You're scaffolding a repo and want the user's first session to come up
  with the right windows already in place.
- A template needs a new window (e.g. add `agent [ai]` to every Go
  service).

## Cheat sheet

All commands operate on `~/.config/ide/environments.json` (or the
`$XDG_CONFIG_HOME` / platform equivalent). They print human-readable
output on success and a non-zero exit on error.

### Environments

```bash
ide env list
ide env show <name>
ide env add    <name> [--root PATH] [--db CONN] [--folder NAME] [--template NAME]
ide env set    <name> [--root PATH] [--db CONN] [--folder NAME]
ide env rename <old> <new>
ide env rm     <name>
```

### Windows inside an environment

```bash
ide env window list <env>
ide env window add  <env> <window> [--cmd CMD] [--cwd CWD]
ide env window set  <env> <window> [--name NEW] [--cmd CMD] [--cwd CWD]
ide env window rm   <env> <window>
```

### Templates (reusable window sets)

```bash
ide template list
ide template show   <name>
ide template add    <name>
ide template rename <old> <new>
ide template rm     <name>

ide template window list <template>
ide template window add  <template> <window> [--cmd CMD] [--cwd CWD]
ide template window set  <template> <window> [--name NEW] [--cmd CMD] [--cwd CWD]
ide template window rm   <template> <window>
```

## Recipes

### Add a new worktree as its own environment

The user is working on `feature/checkout` in a worktree at
`~/code/shop/wt-checkout` and wants `ide` to manage it the same way as the
main checkout:

```bash
ide env add checkout-wt \
  --root ~/code/shop/wt-checkout \
  --template go-service
ide env window add checkout-wt agent --cmd claude
```

Then tell the user: "pick **checkout-wt** in `ide` and hit `enter`".

### Swap a window's command without opening the TUI

```bash
ide env window set api server --cmd "air -c .air.toml"
```

### Add an AI window to every Go service template

```bash
ide template window add go-service "agent [ai]" --cmd claude
```

The `[ai]` tag makes `ide` track the window's agent status (`cooking` /
`awaiting input` / `idle`).

### Inspect before changing

`ide env show <name>` and `ide template show <name>` dump the current
record. Use them to confirm a change before writing, or to read existing
layouts when deriving a new one.

## Conventions for agents

- **Never edit the JSON file directly.** The CLI handles validation and
  formatting; hand-edits can desync against an open TUI.
- **Quote commands with spaces.** `--cmd "go test ./..."` not
  `--cmd go test ./...`.
- **Tag AI windows with `[ai]`** in the window name when launching an
  agent CLI that isn't auto-detected (`claude`, `codex`, `aider`,
  `cursor-agent`, `gemini`, `opencode` are detected automatically).
- **Tell the user the next step.** The CLI only edits config — the user
  still needs to attach in `ide` (or run `r r` to rebuild a running
  session against the new layout).

## Reference

- Config file: `~/.config/ide/environments.json` (path follows
  `os.UserConfigDir`; see the README for macOS / Windows locations).
- Full CLI listing: `ide --help`.
