package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
