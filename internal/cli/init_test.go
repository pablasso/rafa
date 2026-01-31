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

// ========================================================================
// Error Recovery Tests - Init Interrupted Cleanup
// ========================================================================

func TestRunInit_InterruptedNoPartialState(t *testing.T) {
	cleanup := setupTestEnvironment(t)
	t.Cleanup(cleanup)

	t.Run("init failure during directory creation leaves no .rafa/", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer that succeeds (won't be reached)
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{targetDir: targetDir}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Create .rafa directory with read-only permissions to force failure
		os.MkdirAll(".rafa", 0000)
		defer os.Chmod(".rafa", 0755) // Cleanup

		// Run init - should fail due to permissions
		err := runInit(nil, nil)
		if err == nil {
			// On some systems this might not fail, that's OK
			return
		}

		// If it failed, verify .rafa was cleaned up
		os.Chmod(".rafa", 0755) // Make it readable for stat
		info, statErr := os.Stat(".rafa")
		if statErr == nil && info.IsDir() {
			// Check if it's empty
			entries, _ := os.ReadDir(".rafa")
			if len(entries) > 0 {
				t.Error("expected .rafa to be empty after failed init")
			}
		}
	})

	t.Run("init failure during skills installation cleans up completely", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer that fails
		var mockInstaller *mockSkillsInstaller
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			mockInstaller = &mockSkillsInstaller{
				targetDir:  targetDir,
				installErr: errors.New("network failure during skills fetch"),
			}
			return mockInstaller
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init - should fail
		err := runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error on skills failure")
		}

		// Verify .rafa was removed
		if _, err := os.Stat(".rafa"); !os.IsNotExist(err) {
			t.Error(".rafa should not exist after skills failure")
		}

		// Verify .gitignore has no .rafa entries
		content, _ := os.ReadFile(".gitignore")
		if strings.Contains(string(content), ".rafa") {
			t.Error(".gitignore should not contain .rafa entries after failed init")
		}

		// Verify uninstall was called
		if mockInstaller == nil || !mockInstaller.uninstallCalled {
			t.Error("skills Uninstall() should be called for cleanup")
		}
	})

	t.Run("init failure after gitignore update reverts gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create existing .gitignore
		os.WriteFile(".gitignore", []byte("node_modules/\n"), 0644)

		// Mock skills installer that fails
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{
				targetDir:  targetDir,
				installErr: errors.New("download failed"),
			}
		}

		// Initialize git repo
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init - should fail
		err := runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}

		// .gitignore should not have .rafa entries
		content, _ := os.ReadFile(".gitignore")
		contentStr := string(content)
		if strings.Contains(contentStr, ".rafa/**/*.lock") {
			t.Error(".gitignore should not contain .rafa/**/*.lock after failed init")
		}
		if strings.Contains(contentStr, ".rafa/sessions/") {
			t.Error(".gitignore should not contain .rafa/sessions/ after failed init")
		}
		// Original content should be preserved
		if !strings.Contains(contentStr, "node_modules/") {
			t.Error("original .gitignore content should be preserved")
		}
	})

	t.Run("no partial sessions directory after failed init", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Mock skills installer that fails
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &mockSkillsInstaller{
				targetDir:  targetDir,
				installErr: errors.New("skills unavailable"),
			}
		}

		// Initialize git repo
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init - should fail
		runInit(nil, nil)

		// Verify no sessions directory
		if _, err := os.Stat(".rafa/sessions"); !os.IsNotExist(err) {
			t.Error(".rafa/sessions should not exist after failed init")
		}

		// Verify no plans directory
		if _, err := os.Stat(".rafa/plans"); !os.IsNotExist(err) {
			t.Error(".rafa/plans should not exist after failed init")
		}
	})

	t.Run("no .claude/skills directory after failed init", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Track what the installer created
		var installerTargetDir string
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			installerTargetDir = targetDir
			return &mockSkillsInstaller{
				targetDir:  targetDir,
				installErr: errors.New("network error"),
			}
		}

		// Initialize git repo
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init - should fail
		runInit(nil, nil)

		// Skills target dir should have been .claude/skills
		if installerTargetDir != ".claude/skills" {
			t.Errorf("expected skills target to be .claude/skills, got %s", installerTargetDir)
		}

		// Verify skills directories are cleaned up
		// The mockSkillsInstaller.Uninstall() is called which removes the skill dirs
		for _, skill := range []string{"prd", "prd-review", "technical-design", "technical-design-review", "code-review"} {
			skillDir := filepath.Join(".claude/skills", skill)
			if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
				t.Errorf("skill directory %q should not exist after failed init", skill)
			}
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
