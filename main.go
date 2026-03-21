package main

import (
	"ide/cmd"
	"os"
)

func main() {
	os.Exit(exec())
}

func exec() int {
	err := cmd.RunIde()
	if err != nil {
		return 1
	}
	return 0
}
