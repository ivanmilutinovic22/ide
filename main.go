package main

import (
	"ide/cli"
	"os"
)

func main() {
	os.Exit(cli.Exec(os.Args[1:]))
}
