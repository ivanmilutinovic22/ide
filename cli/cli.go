package cmd

import (
	"flag"
	"fmt"
	"ide/run"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func Exec(args []string) int {
	flag.Parse()

	// run main cmd i.e ide
	if len(args) == 0 {
		p := tea.NewProgram(initialModel())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			return 1
		}
		return 0
	}

	switch os.Args[1] {
	case "import-sessions":
		run.ImportSessions()
	default:
		fmt.Println("unknown command:", os.Args[1])
		return 0
	}

	// no suitable cmd
	return 1
}
