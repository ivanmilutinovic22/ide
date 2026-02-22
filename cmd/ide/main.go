package main

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/terminal"
	"ide/internal/ui"
)

func main() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("/tmp/ide-debug.log", "ide")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	} else {
		log.SetOutput(io.Discard)
	}

	// Query the terminal background color before bubbletea takes over stdin.
	termBG := terminal.QueryBackgroundColor()
	log.Printf("terminal background color: %q", termBG)

	p := tea.NewProgram(ui.NewModel(termBG), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
