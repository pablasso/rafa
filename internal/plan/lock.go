package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const lockFileName = "run.lock"

// PlanLock manages a lock file to prevent concurrent runs of the same plan.
type PlanLock struct {
	path string
}

// NewPlanLock creates a new lock manager for the given plan directory.
func NewPlanLock(planDir string) *PlanLock {
	return &PlanLock{
		path: filepath.Join(planDir, lockFileName),
	}
}

// Acquire attempts to acquire the lock.
// Returns an error if the lock is held by another running process.
// Stale locks (from dead processes) are automatically cleaned up.
func (l *PlanLock) Acquire() error {
	// Try atomic creation with O_EXCL
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err == nil {
		// Lock acquired - write our PID
		_, writeErr := fmt.Fprintf(f, "%d", os.Getpid())
		f.Close()
		if writeErr != nil {
			os.Remove(l.path)
			return fmt.Errorf("failed to write lock file: %w", writeErr)
		}
		return nil
	}

	// If error is not "file exists", return it
	if !os.IsExist(err) {
		return fmt.Errorf("failed to create lock file: %w", err)
	}

	// Lock file exists - check if it's stale
	data, readErr := os.ReadFile(l.path)
	if readErr != nil {
		return fmt.Errorf("failed to read existing lock file: %w", readErr)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, parseErr := strconv.Atoi(pidStr)
	if parseErr != nil {
		// Invalid PID in lock file - treat as stale
		if removeErr := os.Remove(l.path); removeErr != nil {
			return fmt.Errorf("failed to remove invalid lock file: %w", removeErr)
		}
		return l.retryAcquire()
	}

	// Check if process is still running
	if processExists(pid) {
		return fmt.Errorf("plan is already running (PID %d)", pid)
	}

	// Process is dead - remove stale lock and retry
	if removeErr := os.Remove(l.path); removeErr != nil {
		return fmt.Errorf("failed to remove stale lock file: %w", removeErr)
	}

	return l.retryAcquire()
}

// retryAcquire attempts to acquire the lock after removing a stale lock.
// Only tries once to avoid infinite loops.
func (l *PlanLock) retryAcquire() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("lock acquired by another process during retry")
		}
		return fmt.Errorf("failed to create lock file on retry: %w", err)
	}

	_, writeErr := fmt.Fprintf(f, "%d", os.Getpid())
	f.Close()
	if writeErr != nil {
		os.Remove(l.path)
		return fmt.Errorf("failed to write lock file on retry: %w", writeErr)
	}
	return nil
}

// Release removes the lock file.
// Returns nil if the lock file doesn't exist (idempotent).
func (l *PlanLock) Release() error {
	err := os.Remove(l.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}
	return nil
}

// IsLocked reports whether the lock is currently held by a live process.
// If the lock file is stale or invalid, it is removed and false is returned.
func (l *PlanLock) IsLocked() (bool, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read existing lock file: %w", err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, parseErr := strconv.Atoi(pidStr)
	if parseErr != nil {
		// Invalid PID in lock file - treat as stale
		if removeErr := os.Remove(l.path); removeErr != nil && !os.IsNotExist(removeErr) {
			return false, fmt.Errorf("failed to remove invalid lock file: %w", removeErr)
		}
		return false, nil
	}

	// Process is still running.
	if processExists(pid) {
		return true, nil
	}

	// Process is dead - remove stale lock.
	if removeErr := os.Remove(l.path); removeErr != nil && !os.IsNotExist(removeErr) {
		return false, fmt.Errorf("failed to remove stale lock file: %w", removeErr)
	}

	return false, nil
}

// processExists checks if a process with the given PID is running.
// Uses kill with signal 0, which checks for process existence without sending a signal.
func processExists(pid int) bool {
	if pid == os.Getpid() {
		return true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't send a signal, just checks if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
