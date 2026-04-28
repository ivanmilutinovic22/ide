// Package run holds the runnable Bubble Tea entrypoints for the ide binary.
// Each exported function (Main, Search) returns a process exit code so the
// dispatcher can stay branch-free of os.Exit.
package run

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/terminal"
	"ide/internal/ui"
)

// Main launches the primary TUI (environments / templates / details).
func Main() int {
	if exit, ok := setupDebugLog(); !ok {
		return exit
	}

	termBG := terminal.QueryBackgroundColor()
	log.Printf("terminal background color: %q", termBG)

	p := tea.NewProgram(ui.NewModel(termBG), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// setupDebugLog enables the bubbletea debug log when DEBUG is set, otherwise
// silences the log package. Returns (exitCode, ok); ok=false means caller
// should return exitCode immediately.
func setupDebugLog() (int, bool) {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("/tmp/ide-debug.log", "ide")
		if err != nil {
			fmt.Fprintln(os.Stderr, "fatal:", err)
			return 1, false
		}
		// File handle deliberately leaks to process end; bubbletea owns
		// flushing on exit.
		_ = f
		return 0, true
	}
	log.SetOutput(io.Discard)
	return 0, true
}
