package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pablasso/rafa/internal/plan"
)

func TestValidateInputs(t *testing.T) {
	t.Run("missing .rafa/ returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create a valid markdown file
		mdFile := filepath.Join(tmpDir, "design.md")
		os.WriteFile(mdFile, []byte("# Design"), 0644)

		opts := CreateOptions{
			FilePath: mdFile,
			DryRun:   false,
		}

		err := validateInputs(opts)
		if err == nil {
			t.Error("expected error for missing .rafa/, got nil")
		}
		if err != nil && err.Error() != "rafa not initialized. Run `rafa init` first" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("dry-run skips .rafa/ check", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create a valid markdown file but no .rafa/
		mdFile := filepath.Join(tmpDir, "design.md")
		os.WriteFile(mdFile, []byte("# Design"), 0644)

		opts := CreateOptions{
			FilePath: mdFile,
			DryRun:   true,
		}

		err := validateInputs(opts)
		if err != nil {
			t.Errorf("dry-run should skip .rafa/ check, got error: %v", err)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ but no file
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		opts := CreateOptions{
			FilePath: filepath.Join(tmpDir, "nonexistent.md"),
			DryRun:   false,
		}

		err := validateInputs(opts)
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	t.Run("non-markdown file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ and a non-markdown file
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)
		txtFile := filepath.Join(tmpDir, "design.txt")
		os.WriteFile(txtFile, []byte("some content"), 0644)

		opts := CreateOptions{
			FilePath: txtFile,
			DryRun:   false,
		}

		err := validateInputs(opts)
		if err == nil {
			t.Error("expected error for non-markdown file, got nil")
		}
	})

	t.Run("empty file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ and an empty markdown file
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)
		emptyFile := filepath.Join(tmpDir, "empty.md")
		os.WriteFile(emptyFile, []byte(""), 0644)

		opts := CreateOptions{
			FilePath: emptyFile,
			DryRun:   false,
		}

		err := validateInputs(opts)
		if err == nil {
			t.Error("expected error for empty file, got nil")
		}
	})

	t.Run("valid inputs pass", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ and a valid markdown file
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)
		mdFile := filepath.Join(tmpDir, "design.md")
		os.WriteFile(mdFile, []byte("# Design Document"), 0644)

		opts := CreateOptions{
			FilePath: mdFile,
			DryRun:   false,
		}

		err := validateInputs(opts)
		if err != nil {
			t.Errorf("expected no error for valid inputs, got: %v", err)
		}
	})
}

func TestDeterminePlanBaseName(t *testing.T) {
	t.Run("--name flag takes precedence", func(t *testing.T) {
		opts := CreateOptions{
			FilePath: "/path/to/my-design.md",
			Name:     "Custom Name",
		}
		extracted := &plan.TaskExtractionResult{
			Name: "AI Extracted Name",
		}

		result := determinePlanBaseName(opts, extracted)
		if result != "custom-name" {
			t.Errorf("got %q, want %q", result, "custom-name")
		}
	})

	t.Run("AI-extracted name used if no flag", func(t *testing.T) {
		opts := CreateOptions{
			FilePath: "/path/to/my-design.md",
			Name:     "",
		}
		extracted := &plan.TaskExtractionResult{
			Name: "AI Extracted Name",
		}

		result := determinePlanBaseName(opts, extracted)
		if result != "ai-extracted-name" {
			t.Errorf("got %q, want %q", result, "ai-extracted-name")
		}
	})

	t.Run("filename used as fallback", func(t *testing.T) {
		opts := CreateOptions{
			FilePath: "/path/to/technical-design.md",
			Name:     "",
		}
		extracted := &plan.TaskExtractionResult{
			Name: "",
		}

		result := determinePlanBaseName(opts, extracted)
		if result != "technical-design" {
			t.Errorf("got %q, want %q", result, "technical-design")
		}
	})

	t.Run("filename with spaces converted to kebab-case", func(t *testing.T) {
		opts := CreateOptions{
			FilePath: "/path/to/My Design Document.md",
			Name:     "",
		}
		extracted := &plan.TaskExtractionResult{
			Name: "",
		}

		result := determinePlanBaseName(opts, extracted)
		if result != "my-design-document" {
			t.Errorf("got %q, want %q", result, "my-design-document")
		}
	})
}

func TestNormalizeSourcePath(t *testing.T) {
	t.Run("absolute path converted to relative", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for macOS (/var -> /private/var)
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ to establish repo root
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		// Create a file in a subdirectory
		subDir := filepath.Join(tmpDir, "docs")
		os.MkdirAll(subDir, 0755)
		absPath := filepath.Join(subDir, "design.md")
		os.WriteFile(absPath, []byte("# Design"), 0644)

		result := normalizeSourcePath(absPath)
		expected := filepath.Join("docs", "design.md")
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("already-relative path unchanged", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ to establish repo root
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		// Create a file
		os.WriteFile(filepath.Join(tmpDir, "design.md"), []byte("# Design"), 0644)

		result := normalizeSourcePath("design.md")
		if result != "design.md" {
			t.Errorf("got %q, want %q", result, "design.md")
		}
	})

	t.Run("path outside repo returns original when no .rafa/", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// No .rafa/ directory - should return original path
		outsidePath := "/some/other/path/design.md"

		result := normalizeSourcePath(outsidePath)
		if result != outsidePath {
			t.Errorf("got %q, want %q", result, outsidePath)
		}
	})
}

func TestFindRepoRoot(t *testing.T) {
	t.Run("finds .rafa/ in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for macOS (/var -> /private/var)
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/ in current directory
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		root, err := findRepoRoot()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("got %q, want %q", root, tmpDir)
		}
	})

	t.Run("finds .rafa/ in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for macOS (/var -> /private/var)
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originalWd, _ := os.Getwd()

		// Create .rafa/ in root
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		// Create a subdirectory and change to it
		subDir := filepath.Join(tmpDir, "src", "components")
		os.MkdirAll(subDir, 0755)
		os.Chdir(subDir)
		defer os.Chdir(originalWd)

		root, err := findRepoRoot()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root != tmpDir {
			t.Errorf("got %q, want %q", root, tmpDir)
		}
	})

	t.Run("returns error when .rafa/ not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// No .rafa/ directory

		_, err := findRepoRoot()
		if err == nil {
			t.Error("expected error when .rafa/ not found, got nil")
		}
	})
}

func TestAssemblePlan(t *testing.T) {
	t.Run("plan ID is generated", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/plans/ for ResolvePlanName
		os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)

		opts := CreateOptions{
			FilePath: "design.md",
			Name:     "test-plan",
		}
		extracted := &plan.TaskExtractionResult{
			Description: "A test plan",
			Tasks: []plan.ExtractedTask{
				{
					Title:              "Task 1",
					Description:        "Do something",
					AcceptanceCriteria: []string{"It works"},
				},
			},
		}

		p, err := assemblePlan(opts, extracted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// ID should be 6 characters
		if len(p.ID) != 6 {
			t.Errorf("plan ID should be 6 characters, got %d: %q", len(p.ID), p.ID)
		}
	})

	t.Run("tasks get sequential IDs", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)

		opts := CreateOptions{
			FilePath: "design.md",
			Name:     "test-plan",
		}
		extracted := &plan.TaskExtractionResult{
			Description: "A test plan",
			Tasks: []plan.ExtractedTask{
				{Title: "Task 1", Description: "First", AcceptanceCriteria: []string{"AC1"}},
				{Title: "Task 2", Description: "Second", AcceptanceCriteria: []string{"AC2"}},
				{Title: "Task 3", Description: "Third", AcceptanceCriteria: []string{"AC3"}},
			},
		}

		p, err := assemblePlan(opts, extracted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedIDs := []string{"t01", "t02", "t03"}
		for i, task := range p.Tasks {
			if task.ID != expectedIDs[i] {
				t.Errorf("task %d ID: got %q, want %q", i, task.ID, expectedIDs[i])
			}
		}
	})

	t.Run("status is set to pending", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)

		opts := CreateOptions{
			FilePath: "design.md",
			Name:     "test-plan",
		}
		extracted := &plan.TaskExtractionResult{
			Description: "A test plan",
			Tasks: []plan.ExtractedTask{
				{Title: "Task 1", Description: "Do something", AcceptanceCriteria: []string{"AC"}},
			},
		}

		p, err := assemblePlan(opts, extracted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Plan status should be not_started
		if p.Status != plan.PlanStatusNotStarted {
			t.Errorf("plan status: got %q, want %q", p.Status, plan.PlanStatusNotStarted)
		}

		// Task status should be pending
		for i, task := range p.Tasks {
			if task.Status != plan.TaskStatusPending {
				t.Errorf("task %d status: got %q, want %q", i, task.Status, plan.TaskStatusPending)
			}
		}
	})

	t.Run("source path is normalized", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Resolve symlinks for macOS (/var -> /private/var)
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)
		docsDir := filepath.Join(tmpDir, "docs")
		os.MkdirAll(docsDir, 0755)

		// Use absolute path in options
		absPath := filepath.Join(docsDir, "design.md")
		os.WriteFile(absPath, []byte("# Design"), 0644)

		opts := CreateOptions{
			FilePath: absPath,
			Name:     "test-plan",
		}
		extracted := &plan.TaskExtractionResult{
			Description: "A test plan",
			Tasks: []plan.ExtractedTask{
				{Title: "Task 1", Description: "Do something", AcceptanceCriteria: []string{"AC"}},
			},
		}

		p, err := assemblePlan(opts, extracted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Source path should be relative
		expected := filepath.Join("docs", "design.md")
		if p.SourceFile != expected {
			t.Errorf("source file: got %q, want %q", p.SourceFile, expected)
		}
	})

	t.Run("preserves task details from extraction", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)

		opts := CreateOptions{
			FilePath: "design.md",
			Name:     "test-plan",
		}
		extracted := &plan.TaskExtractionResult{
			Name:        "Extracted Plan Name",
			Description: "Plan description from AI",
			Tasks: []plan.ExtractedTask{
				{
					Title:              "Implement feature X",
					Description:        "Detailed description of feature X",
					AcceptanceCriteria: []string{"Tests pass", "Documentation updated"},
				},
			},
		}

		p, err := assemblePlan(opts, extracted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if p.Description != extracted.Description {
			t.Errorf("description: got %q, want %q", p.Description, extracted.Description)
		}

		task := p.Tasks[0]
		if task.Title != extracted.Tasks[0].Title {
			t.Errorf("task title: got %q, want %q", task.Title, extracted.Tasks[0].Title)
		}
		if task.Description != extracted.Tasks[0].Description {
			t.Errorf("task description: got %q, want %q", task.Description, extracted.Tasks[0].Description)
		}
		if len(task.AcceptanceCriteria) != len(extracted.Tasks[0].AcceptanceCriteria) {
			t.Errorf("acceptance criteria count: got %d, want %d",
				len(task.AcceptanceCriteria), len(extracted.Tasks[0].AcceptanceCriteria))
		}
		if task.Attempts != 0 {
			t.Errorf("attempts should be 0, got %d", task.Attempts)
		}
	})
}
