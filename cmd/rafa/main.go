package main

import (
	"os"

	"github.com/pablasso/rafa/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
