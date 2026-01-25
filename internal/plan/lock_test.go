package plan

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPlanLock_Acquire_Success(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewPlanLock(tmpDir)
	err := lock.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify lock file exists with our PID
	lockPath := filepath.Join(tmpDir, lockFileName)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("failed to parse PID from lock file: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("lock file PID mismatch: got %d, want %d", pid, os.Getpid())
	}
}

func TestPlanLock_Acquire_AlreadyLocked(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a lock file with current process PID (simulating another running process)
	lockPath := filepath.Join(tmpDir, lockFileName)
	err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Try to acquire lock - should fail because our process is running
	lock := NewPlanLock(tmpDir)
	err = lock.Acquire()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.HasPrefix(err.Error(), "plan is already running") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPlanLock_Acquire_StaleLock(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a lock file with a non-existent PID
	// PID 99999999 is unlikely to exist
	lockPath := filepath.Join(tmpDir, lockFileName)
	err := os.WriteFile(lockPath, []byte("99999999"), 0644)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Acquire should succeed after removing stale lock
	lock := NewPlanLock(tmpDir)
	err = lock.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify lock file now has our PID
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("failed to parse PID from lock file: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("lock file PID mismatch: got %d, want %d", pid, os.Getpid())
	}
}

func TestPlanLock_Acquire_InvalidLockFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a lock file with invalid content
	lockPath := filepath.Join(tmpDir, lockFileName)
	err := os.WriteFile(lockPath, []byte("not-a-pid"), 0644)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Acquire should succeed after removing invalid lock
	lock := NewPlanLock(tmpDir)
	err = lock.Acquire()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify lock file now has our PID
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("failed to parse PID from lock file: %v", err)
	}

	if pid != os.Getpid() {
		t.Errorf("lock file PID mismatch: got %d, want %d", pid, os.Getpid())
	}
}

func TestPlanLock_Acquire_RaceCondition(t *testing.T) {
	tmpDir := t.TempDir()

	const numGoroutines = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock := NewPlanLock(tmpDir)
			if err := lock.Acquire(); err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// Exactly one goroutine should have succeeded
	if count := successCount.Load(); count != 1 {
		t.Errorf("expected exactly 1 successful acquire, got %d", count)
	}
}

func TestPlanLock_Release(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewPlanLock(tmpDir)

	// Acquire the lock first
	err := lock.Acquire()
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Verify lock file exists
	lockPath := filepath.Join(tmpDir, lockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}

	// Release the lock
	err = lock.Release()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should be removed after release")
	}
}

func TestPlanLock_Release_NotHeld(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewPlanLock(tmpDir)

	// Release without acquiring - should not error
	err := lock.Release()
	if err != nil {
		t.Errorf("unexpected error when releasing unheld lock: %v", err)
	}
}

func TestPlanLock_AcquireAfterRelease(t *testing.T) {
	tmpDir := t.TempDir()

	lock := NewPlanLock(tmpDir)

	// Acquire
	err := lock.Acquire()
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Release
	err = lock.Release()
	if err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Should be able to acquire again
	err = lock.Acquire()
	if err != nil {
		t.Fatalf("failed to re-acquire lock after release: %v", err)
	}
}
