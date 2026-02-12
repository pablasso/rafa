package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/pablasso/rafa/internal/tui"
	"github.com/pablasso/rafa/internal/version"
)

func main() {
	// Set up crash signal handling for better crash diagnostics
	crashChan := make(chan os.Signal, 1)
	signal.Notify(crashChan, syscall.SIGSEGV, syscall.SIGBUS, syscall.SIGABRT)
	go func() {
		sig := <-crashChan
		writeCrashLogToHome(sig)
		os.Exit(1)
	}()

	parsed, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}
	if parsed.ShowHelp {
		fmt.Fprintln(os.Stdout, parsed.HelpText)
		os.Exit(0)
	}
	if parsed.ShowVersion {
		fmt.Fprintf(os.Stdout, "Rafa %s\nCommit: %s\nBuilt:  %s\n", version.Version, version.CommitSHA, version.BuildDate)
		os.Exit(0)
	}

	if err := tui.Run(parsed.Options); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// writeCrashLogToHome writes crash signal information to ~/.rafa/crash.log
func writeCrashLogToHome(sig os.Signal) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	rafaDir := filepath.Join(home, ".rafa")
	if err := os.MkdirAll(rafaDir, 0755); err != nil {
		return
	}

	crashLog := filepath.Join(rafaDir, "crash.log")
	f, err := os.OpenFile(crashLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// Capture all goroutines' stacks, not just the signal handler's
	buf := make([]byte, 1024*1024) // 1MB buffer
	n := runtime.Stack(buf, true)  // true = all goroutines

	fmt.Fprintf(f, "=== Crash at: %s ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "Signal: %v\n\n", sig)
	fmt.Fprintf(f, "All goroutines:\n%s\n\n", buf[:n])
}
