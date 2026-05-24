// Package cli implements the non-TUI subcommands: CRUD over environments,
// templates, and windows in ~/.config/ide/environments.json.
package cli

import (
	"fmt"
	"io"
	"os"
)

// Subcommands is the set of first-arg keywords that route into the CLI
// rather than launching the TUI. Anything else is left to the caller.
var Subcommands = map[string]bool{
	"env":      true,
	"template": true,
}

const Usage = `CLI commands (read/modify ~/.config/ide/environments.json):

  ide env list
  ide env show <name>
  ide env add <name> [--root PATH] [--db CONN] [--folder NAME] [--template NAME]
  ide env set <name> [--root PATH] [--db CONN] [--folder NAME]
  ide env rename <old> <new>
  ide env rm <name>

  ide env window list <env>
  ide env window add <env> <window> [--cmd CMD] [--cwd CWD]
  ide env window set <env> <window> [--name NEW] [--cmd CMD] [--cwd CWD]
  ide env window rm <env> <window>

  ide template list
  ide template show <name>
  ide template add <name>
  ide template rename <old> <new>
  ide template rm <name>

  ide template window list <template>
  ide template window add <template> <window> [--cmd CMD] [--cwd CWD]
  ide template window set <template> <window> [--name NEW] [--cmd CMD] [--cwd CWD]
  ide template window rm <template> <window>
`

// Dispatch routes a CLI subcommand. args is os.Args[1:]. Returns a process
// exit code. Caller must have already verified args[0] is in Subcommands.
func Dispatch(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, Usage)
		return 2
	}
	switch args[0] {
	case "env":
		return dispatchEnv(args[1:])
	case "template":
		return dispatchTemplate(args[1:])
	}
	fmt.Fprintf(os.Stderr, "ide: unknown subcommand %q\n\n%s", args[0], Usage)
	return 2
}

func errf(w io.Writer, format string, a ...any) int {
	fmt.Fprintf(w, "ide: "+format+"\n", a...)
	return 1
}

func usagef(w io.Writer, format string, a ...any) int {
	fmt.Fprintf(w, format+"\n", a...)
	return 2
}
