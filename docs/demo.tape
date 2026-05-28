# VHS tape — short showcase of ide.
# Run with: vhs docs/demo.tape    (needs `vhs`, `tmux` 3.2+, and ./build/ide)
#
# Uses a throwaway $HOME *and* $TMUX_TMPDIR so neither the real ide config
# nor the real tmux server is touched. A trap on EXIT tears both down.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 70ms

# Fresh empty $HOME + isolated tmux socket, then launch ide.
# We pre-seed environments.json with the Catppuccin Mocha theme so ide
# boots straight into it (no theme picker dance needed).
Hide
Type `export HOME=/tmp/ide-demo-home; export TMUX_TMPDIR=/tmp/ide-demo-tmux; rm -rf "$HOME" "$TMUX_TMPDIR"; mkdir -p "$HOME/Library/Application Support/ide" "$TMUX_TMPDIR"; printf '{"theme":"Catppuccin Mocha","environments":[]}' > "$HOME/Library/Application Support/ide/environments.json"; trap 'tmux kill-server 2>/dev/null; rm -rf "$HOME" "$TMUX_TMPDIR"' EXIT; cd /Users/vladafilipovic/Documents/ide && clear && ./build/ide`
Enter
Sleep 2200ms
Show

# Empty dashboard — let the empty state read.
Sleep 2000ms

# Create a new environment, slowly.
Type "c"
Sleep 1500ms

# Name.
Type@90ms "demo"
Sleep 1200ms
Enter
Sleep 1000ms

# Root path — an existing repo so no folder-create prompt.
Type@65ms "/Users/vladafilipovic/Documents/ide"
Sleep 1500ms
Enter
Sleep 1000ms

# Template picker. Right-arrow flips from "default" to "custom" so we can
# spec windows by hand.
Right
Sleep 1200ms
Enter
Sleep 1000ms

# Custom windows: editor=nvim, lazygit, and a Claude window tagged [ai].
# Format is name=cmd separated by ';'. The [ai] tag triggers ide's
# agent-status detection on that window.
Type@55ms `editor=nvim .;lazygit=lazygit;claude [ai]=claude`
Sleep 2500ms
Enter

# tmux session being created — let the windows render.
Sleep 4000ms

# Cycle panes so the right side updates: Sessions → Agents → Windows.
Tab
Sleep 1800ms
Tab
Sleep 2200ms

# Help overlay reveals the AI-tracking legend (cooking / awaiting).
Type "?"
Sleep 3500ms
Escape
Sleep 800ms

# Quit ide, then exit bash so the EXIT trap fires (tears down demo state).
Type "q"
Sleep 600ms
Hide
Type "exit"
Enter
Sleep 400ms
