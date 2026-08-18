// Package run is the CLI dispatcher: it parses os.Args and invokes the right
// run subcommand from the ide/run package.
package run

import (
	"fmt"
	"os"

	"ide/internal/cli"
	"ide/run"
)

// version is set by the linker (`-ldflags "-X ide/cmd/run.version=..."`)
// for release builds; otherwise it falls back to "dev".
var version = "dev"

const usage = `ide — terminal UI for managing tmux-based dev environments

Usage:
  ide              Launch the main TUI
  ide --search     Open the fuzzy-search popup (used by tmux prefix+s)
  ide --windows    Open the current-session window switcher (used by tmux prefix+w)
  ide --help       Show this message
  ide --version    Print the version

` + cli.Usage

// Dispatch parses argv and runs the matching subcommand. Returns a process
// exit code.
func Dispatch() int {
	args := os.Args[1:]

	// Route CLI subcommands (env, template, ...) before treating args as TUI
	// flags. This lets `ide env list` etc. coexist with `ide --search`.
	if len(args) > 0 && cli.Subcommands[args[0]] {
		return cli.Dispatch(args)
	}

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
		case "--windows":
			mode = "windows"
		default:
			fmt.Fprintf(os.Stderr, "ide: unknown argument %q\n\n%s", a, usage)
			return 2
		}
	}

	if mode == "search" {
		return run.Search()
	}
	if mode == "windows" {
		return run.Windows()
	}
	return run.Main()
}
