package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunDeinit(t *testing.T) {
	t.Run("deinit when not initialized fails", func(t *testing.T) {
		// Create a temp dir without .rafa
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Run deinit command
		err := runDeinit(nil, nil)
		if err == nil {
			t.Fatal("expected error when not initialized, got nil")
		}

		expectedErr := "rafa is not initialized in this repository"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("deinit when .rafa is a file fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa as a file instead of directory
		if err := os.WriteFile(".rafa", []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create .rafa file: %v", err)
		}

		// Run deinit command
		err := runDeinit(nil, nil)
		if err == nil {
			t.Fatal("expected error when .rafa is a file, got nil")
		}

		expectedErr := ".rafa exists but is not a directory"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("deinit with force removes directory and updates gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa directory structure
		rafaPath := filepath.Join(tmpDir, ".rafa")
		plansPath := filepath.Join(rafaPath, "plans")
		if err := os.MkdirAll(plansPath, 0755); err != nil {
			t.Fatalf("failed to create .rafa/plans: %v", err)
		}

		// Create a test plan directory
		testPlan := filepath.Join(plansPath, "test-plan")
		if err := os.MkdirAll(testPlan, 0755); err != nil {
			t.Fatalf("failed to create test plan: %v", err)
		}

		// Create a test file in the plan
		testFile := filepath.Join(testPlan, "plan.json")
		if err := os.WriteFile(testFile, []byte(`{"name":"test"}`), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create .gitignore with rafa entry and another entry
		if err := os.WriteFile(".gitignore", []byte("other-entry\n.rafa/**/*.lock\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		// Set force flag
		oldForce := deinitForce
		deinitForce = true
		defer func() { deinitForce = oldForce }()

		// Run deinit command
		err := runDeinit(nil, nil)
		if err != nil {
			t.Fatalf("runDeinit failed: %v", err)
		}

		// Verify .rafa directory was removed
		if _, err := os.Stat(".rafa"); err == nil {
			t.Error("expected .rafa directory to be removed")
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking .rafa: %v", err)
		}

		// Verify .gitignore no longer contains rafa entry
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to still exist: %v", err)
		}
		if string(content) != "other-entry\n" {
			t.Errorf("expected gitignore to have rafa entry removed, got %q", string(content))
		}
	})
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0B"},
		{1, "1B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{10240, "10.0KB"},
		{1048576, "1.0MB"},
		{1572864, "1.5MB"},
		{10485760, "10.0MB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestCalculateDirStats(t *testing.T) {
	t.Run("counts plans correctly", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .rafa/plans structure with some plan directories
		plansDir := filepath.Join(tmpDir, "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}

		// Create 3 plan directories
		for _, planName := range []string{"plan1", "plan2", "plan3"} {
			planPath := filepath.Join(plansDir, planName)
			if err := os.MkdirAll(planPath, 0755); err != nil {
				t.Fatalf("failed to create plan %s: %v", planName, err)
			}
		}

		planCount, _, err := calculateDirStats(tmpDir)
		if err != nil {
			t.Fatalf("calculateDirStats failed: %v", err)
		}

		if planCount != 3 {
			t.Errorf("expected 3 plans, got %d", planCount)
		}
	})

	t.Run("calculates total size correctly", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create plans directory
		plansDir := filepath.Join(tmpDir, "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}

		// Create files with known sizes
		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(plansDir, "file2.txt")

		// 100 bytes
		if err := os.WriteFile(file1, make([]byte, 100), 0644); err != nil {
			t.Fatalf("failed to create file1: %v", err)
		}
		// 200 bytes
		if err := os.WriteFile(file2, make([]byte, 200), 0644); err != nil {
			t.Fatalf("failed to create file2: %v", err)
		}

		_, totalSize, err := calculateDirStats(tmpDir)
		if err != nil {
			t.Fatalf("calculateDirStats failed: %v", err)
		}

		if totalSize != 300 {
			t.Errorf("expected total size 300, got %d", totalSize)
		}
	})

	t.Run("handles empty plans directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .rafa/plans structure with no plans
		plansDir := filepath.Join(tmpDir, "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}

		planCount, totalSize, err := calculateDirStats(tmpDir)
		if err != nil {
			t.Fatalf("calculateDirStats failed: %v", err)
		}

		if planCount != 0 {
			t.Errorf("expected 0 plans, got %d", planCount)
		}
		if totalSize != 0 {
			t.Errorf("expected 0 total size, got %d", totalSize)
		}
	})

	t.Run("handles missing plans directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Don't create plans directory

		// Should not error, just return 0 plan count
		planCount, _, err := calculateDirStats(tmpDir)
		if err != nil {
			t.Fatalf("calculateDirStats failed: %v", err)
		}

		if planCount != 0 {
			t.Errorf("expected 0 plans, got %d", planCount)
		}
	})
}
