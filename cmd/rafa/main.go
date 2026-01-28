package main

import (
	"fmt"
	"os"

	"github.com/pablasso/rafa/internal/cli"
	"github.com/pablasso/rafa/internal/tui"
)

func main() {
	// If no args, launch TUI; otherwise route to CLI
	if len(os.Args) == 1 {
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := cli.Execute(); err != nil {
			os.Exit(1)
		}
	}
}
