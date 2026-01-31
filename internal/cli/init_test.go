package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mockSkillsInstaller implements SkillsInstaller for testing.
type mockSkillsInstaller struct {
	targetDir       string
	installCalled   bool
	uninstallCalled bool
	installErr      error
	uninstallErr    error
}

func (m *mockSkillsInstaller) Install() error {
	m.installCalled = true
	if m.installErr != nil {
		return m.installErr
	}
	// Create the skill directories for successful installation
	for _, skill := range []string{"prd", "prd-review", "technical-design", "technical-design-review", "code-review"} {
		skillDir := filepath.Join(m.targetDir, skill)
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+skill), 0644)
	}
	return nil
}

func (m *mockSkillsInstaller) Uninstall() error {
	m.uninstallCalled = true
	if m.uninstallErr != nil {
		return m.uninstallErr
	}
	// Remove skill directories
	for _, skill := range []string{"prd", "prd-review", "technical-design", "technical-design-review", "code-review"} {
		skillDir := filepath.Join(m.targetDir, skill)
		os.RemoveAll(skillDir)
	}
	return nil
}

func setupTestEnvironment(t *testing.T) (cleanup func()) {
	// Mock command execution to avoid slow claude auth checks
	originalCommandFunc := commandFunc
	originalLookPathFunc := lookPathFunc
	originalSkillsFactory := skillsInstallerFactory

	commandFunc = func(name string, args ...string) *exec.Cmd {
		if name == "claude" {
			return exec.Command("true") // instant success for claude commands
		}
		return exec.Command(name, args...) // use real command for git
	}
	lookPathFunc = func(file string) (string, error) {
		return "/usr/bin/" + file, nil // pretend all commands exist
	}

	return func() {
		commandFunc = originalCommandFunc
		lookPathFunc = originalLookPathFunc
		skillsInstallerFactory = originalSkillsFactory
	}
}

func TestRunInit(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	t.Cleanup(cleanup)

	t.Run("successful init creates directories and updates gitignore", func(t *testing.T) {
		// Create a temp dir with git initialized
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer
		var mockInstaller *mockSkillsInstaller
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			mockInstaller = &mockSkillsInstaller{targetDir: targetDir}
			return mockInstaller
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Verify .rafa directory was created
		rafaInfo, err := os.Stat(".rafa")
		if err != nil {
			t.Fatalf("expected .rafa directory to exist, got error: %v", err)
		}
		if !rafaInfo.IsDir() {
			t.Error("expected .rafa to be a directory")
		}

		// Verify .rafa/plans directory was created
		plansInfo, err := os.Stat(filepath.Join(".rafa", "plans"))
		if err != nil {
			t.Fatalf("expected .rafa/plans directory to exist, got error: %v", err)
		}
		if !plansInfo.IsDir() {
			t.Error("expected .rafa/plans to be a directory")
		}

		// Verify .gitignore contains both lock and sessions patterns
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist, got error: %v", err)
		}
		contentStr := string(content)
		if !strings.Contains(contentStr, ".rafa/**/*.lock") {
			t.Errorf("expected .gitignore to contain lock pattern, got %q", contentStr)
		}
		if !strings.Contains(contentStr, ".rafa/sessions/") {
			t.Errorf("expected .gitignore to contain sessions pattern, got %q", contentStr)
		}

		// Verify skills installer was called
		if mockInstaller == nil || !mockInstaller.installCalled {
			t.Error("expected skills installer Install() to be called")
		}
	})

	t.Run("init creates sessions directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{targetDir: targetDir}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Verify .rafa/sessions directory was created
		sessionsInfo, err := os.Stat(filepath.Join(".rafa", "sessions"))
		if err != nil {
			t.Fatalf("expected .rafa/sessions directory to exist, got error: %v", err)
		}
		if !sessionsInfo.IsDir() {
			t.Error("expected .rafa/sessions to be a directory")
		}
	})

	t.Run("init installs skills to .claude/skills", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer that tracks the target directory
		var capturedTargetDir string
		var mockInstaller *mockSkillsInstaller
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			capturedTargetDir = targetDir
			mockInstaller = &mockSkillsInstaller{targetDir: targetDir}
			return mockInstaller
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Verify skills installer was called with correct target directory
		if capturedTargetDir != ".claude/skills" {
			t.Errorf("expected skills to be installed to .claude/skills, got %q", capturedTargetDir)
		}
		if mockInstaller == nil || !mockInstaller.installCalled {
			t.Error("expected skills installer Install() to be called")
		}
	})

	t.Run("init adds sessions to gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{targetDir: targetDir}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Verify .gitignore contains sessions entry
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist, got error: %v", err)
		}
		if !strings.Contains(string(content), ".rafa/sessions/") {
			t.Errorf("expected .gitignore to contain '.rafa/sessions/', got %q", string(content))
		}
	})

	t.Run("init cleans up completely on skills installation failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer that fails
		var mockInstaller *mockSkillsInstaller
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			mockInstaller = &mockSkillsInstaller{
				targetDir:  targetDir,
				installErr: errors.New("skills repo unavailable"),
			}
			return mockInstaller
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command - should fail
		err := runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error when skills installation fails, got nil")
		}

		// Verify error message mentions skills
		if !strings.Contains(err.Error(), "skills") {
			t.Errorf("expected error to mention skills, got %q", err.Error())
		}

		// Verify .rafa directory was cleaned up
		if _, err := os.Stat(".rafa"); err == nil {
			t.Error("expected .rafa directory to be cleaned up after skills installation failure")
		}

		// Verify skills uninstall was called for cleanup
		if mockInstaller == nil || !mockInstaller.uninstallCalled {
			t.Error("expected skills installer Uninstall() to be called for cleanup")
		}

		// Verify no partial .gitignore entries
		content, _ := os.ReadFile(".gitignore")
		if strings.Contains(string(content), ".rafa") {
			t.Error("expected no .rafa entries in .gitignore after failed init")
		}
	})

	t.Run("init fails completely if skills repo is unavailable", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer that fails
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{
				targetDir:  targetDir,
				installErr: errors.New("network error: skills repo unavailable"),
			}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command - should fail
		err := runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error when skills repo is unavailable, got nil")
		}

		// Verify no partial state exists
		if _, err := os.Stat(".rafa"); err == nil {
			t.Error("expected no .rafa directory after failed init")
		}
		if _, err := os.Stat(".claude"); err == nil {
			t.Error("expected no .claude directory after failed init")
		}
	})

	t.Run("init outside git repo fails", func(t *testing.T) {
		// Create a temp dir without git
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{targetDir: targetDir}
		}

		// Run init command
		err := runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error when not in git repo, got nil")
		}

		// Verify it's a PrerequisiteError about git
		prereqErr, ok := err.(*PrerequisiteError)
		if !ok {
			t.Fatalf("expected *PrerequisiteError, got %T: %v", err, err)
		}

		if prereqErr.Check != "Git repository" {
			t.Errorf("expected Check to be 'Git repository', got %q", prereqErr.Check)
		}

		// Verify .rafa was not created
		if _, err := os.Stat(".rafa"); err == nil {
			t.Error("expected .rafa directory to not exist after failed init")
		}
	})

	t.Run("double init fails", func(t *testing.T) {
		// Create a temp dir with git initialized
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{targetDir: targetDir}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// First init should succeed
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("first runInit failed: %v", err)
		}

		// Second init should fail
		err = runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error on second init, got nil")
		}

		expectedErr := "rafa is already initialized in this repository"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})
}

func TestAddToGitignore(t *testing.T) {
	t.Run("creates gitignore if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		err := addToGitignore("test-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist: %v", err)
		}
		if string(content) != "test-entry\n" {
			t.Errorf("expected 'test-entry\\n', got %q", string(content))
		}
	})

	t.Run("appends to existing gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("existing-entry\n"), 0644)

		err := addToGitignore("new-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		expected := "existing-entry\nnew-entry\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("adds newline if file doesn't end with one", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("no-trailing-newline"), 0644)

		err := addToGitignore("new-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		expected := "no-trailing-newline\nnew-entry\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("skips if entry already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("existing-entry\n"), 0644)

		err := addToGitignore("existing-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		if string(content) != "existing-entry\n" {
			t.Errorf("expected unchanged content, got %q", string(content))
		}
	})
}

func TestRemoveFromGitignore(t *testing.T) {
	t.Run("removes entry from gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("keep-me\nremove-me\nalso-keep\n"), 0644)

		err := removeFromGitignore("remove-me")
		if err != nil {
			t.Fatalf("removeFromGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		expected := "keep-me\nalso-keep\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("does nothing if gitignore doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		err := removeFromGitignore("any-entry")
		if err != nil {
			t.Fatalf("removeFromGitignore should not fail: %v", err)
		}
	})

	t.Run("leaves gitignore if only rafa entry would remain", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("only-entry\n"), 0644)

		err := removeFromGitignore("only-entry")
		if err != nil {
			t.Fatalf("removeFromGitignore failed: %v", err)
		}

		// File should still exist (we don't delete it)
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to still exist: %v", err)
		}
		// Content should be unchanged since removing would make it empty
		if string(content) != "only-entry\n" {
			t.Errorf("expected unchanged content when result would be empty, got %q", string(content))
		}
	})
}
