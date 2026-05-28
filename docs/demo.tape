# VHS tape — short showcase of ide.
# Run with: vhs docs/demo.tape    (needs `vhs`, `tmux` 3.2+, and ./build/ide)
#
# Uses a throwaway $HOME *and* $TMUX_TMPDIR so neither the real ide config
# nor the real tmux server is touched. A trap on EXIT tears down both.

Output docs/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms

# Fresh empty $HOME + isolated tmux socket, then launch ide.
Hide
Type `export HOME=/tmp/ide-demo-home; export TMUX_TMPDIR=/tmp/ide-demo-tmux; rm -rf "$HOME" "$TMUX_TMPDIR"; mkdir -p "$HOME/Library/Application Support/ide" "$TMUX_TMPDIR"; trap 'tmux kill-server 2>/dev/null; rm -rf "$HOME" "$TMUX_TMPDIR"' EXIT; cd /Users/vladafilipovic/Documents/ide && clear && ./build/ide`
Enter
Sleep 1800ms
Show

# Empty dashboard — let the empty state read.
Sleep 1500ms

# Create a new environment.
Type "c"
Sleep 1000ms

# Name.
Type "demo"
Sleep 600ms
Enter
Sleep 500ms

# Root path (an existing repo so no folder-create confirm).
Type "/Users/vladafilipovic/Documents/ide"
Sleep 700ms
Enter
Sleep 500ms

# Template field — default template is preselected. Submit.
Enter
Sleep 3000ms

# Cycle panes to show Windows + Agents.
Tab
Sleep 1200ms
Tab
Sleep 1500ms

# Help overlay reveals AI-tracking legend (cooking / awaiting).
Type "?"
Sleep 2800ms
Escape
Sleep 700ms

# Quit ide, then exit bash so the EXIT trap fires (tears down demo state).
Type "q"
Sleep 400ms
Hide
Type "exit"
Enter
Sleep 400ms
