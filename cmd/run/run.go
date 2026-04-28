// Package run is the CLI dispatcher: it parses os.Args and invokes the right
// run subcommand from the ide/run package.
package run

import (
	"os"
	"slices"

	iderun "ide/run"
)

// Dispatch parses argv and runs the matching subcommand. Returns a process
// exit code.
func Dispatch() int {
	args := os.Args[1:]
	if slices.Contains(args, "--search") {
		return iderun.Search()
	}
	return iderun.Main()
}
