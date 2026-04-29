// Package run is the CLI dispatcher: it parses os.Args and invokes the right
// run subcommand from the ide/run package.
package run

import (
	"fmt"
	"os"

	"ide/run"
)

// version is set by the linker (`-ldflags "-X ide/cmd/run.version=..."`)
// for release builds; otherwise it falls back to "dev".
var version = "dev"

const usage = `ide — terminal UI for managing tmux-based dev environments

Usage:
  ide              Launch the main TUI
  ide --search     Open the fuzzy-search popup (used by tmux prefix+a)
  ide --help       Show this message
  ide --version    Print the version
`

// Dispatch parses argv and runs the matching subcommand. Returns a process
// exit code.
func Dispatch() int {
	args := os.Args[1:]

	// Peel off recognized flags. Anything else is an error so users learn
	// about typos instead of silently getting the default behavior.
	mode := "main"
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Print(usage)
			return 0
		case "--version", "-v":
			fmt.Println("ide", version)
			return 0
		case "--search":
			mode = "search"
		default:
			fmt.Fprintf(os.Stderr, "ide: unknown argument %q\n\n%s", a, usage)
			return 2
		}
	}

	if mode == "search" {
		return run.Search()
	}
	return run.Main()
}
