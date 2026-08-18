package run

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/ui"
)

// Windows launches the current-session window switcher popup used by the tmux
// prefix+w keybinding. It reuses the search popup, scoped to the windows of
// the session it was launched from.
func Windows() int {
	if exit, ok := setupDebugLog(); !ok {
		return exit
	}
	p := tea.NewProgram(ui.NewSessionWindowsModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
