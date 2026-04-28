package main

import (
	"os"

	"ide/cmd/run"
)

func main() {
	os.Exit(run.Dispatch())
}
