package cmd

import (
	"flag"
	"fmt"
	"ide/run"
	"os"
)

func Exec(args []string) int {
	flag.Parse()

	// run main cmd i.e ide
	if len(args) == 0 {
		err := run.Ide()
		if err != nil {
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
