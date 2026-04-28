package run

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/ui"
)

// Search launches the fuzzy search popup used by the tmux prefix+a keybinding.
func Search() int {
	if exit, ok := setupDebugLog(); !ok {
		return exit
	}
	p := tea.NewProgram(ui.NewSearchModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
