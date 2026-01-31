package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablasso/rafa/internal/skills"
)

// TestDeinitIntegration tests the end-to-end deinit flow.
func TestDeinitIntegration(t *testing.T) {
	// Save originals
	originalCommandFunc := commandFunc
	originalLookPathFunc := lookPathFunc
	originalSkillsFactory := skillsInstallerFactory

	// Mock command execution to avoid slow claude auth checks
	commandFunc = func(name string, args ...string) *exec.Cmd {
		if name == "claude" {
			return exec.Command("true") // instant success for claude commands
		}
		return exec.Command(name, args...) // use real command for git
	}
	lookPathFunc = func(file string) (string, error) {
		return "/usr/bin/" + file, nil // pretend all commands exist
	}

	t.Cleanup(func() {
		commandFunc = originalCommandFunc
		lookPathFunc = originalLookPathFunc
		skillsInstallerFactory = originalSkillsFactory
	})

	t.Run("end-to-end deinit removes both .rafa/ and skills", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Use integration test skills installer
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &integrationTestSkillsInstaller{
				targetDir: targetDir,
			}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Create a .gitignore with a non-rafa entry so we can verify rafa entries are removed
		// (the removeFromGitignore function won't remove entries that would leave the file empty)
		if err := os.WriteFile(".gitignore", []byte("node_modules/\n"), 0644); err != nil {
			t.Fatalf("failed to create .gitignore: %v", err)
		}

		// Run init command first
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Verify everything was created
		dirsToExist := []string{
			".rafa",
			".rafa/plans",
			".rafa/sessions",
			".claude/skills",
		}
		for _, dir := range dirsToExist {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Fatalf("expected directory %s to exist before deinit", dir)
			}
		}

		// Verify skills were installed
		for _, skill := range skills.RequiredSkills {
			skillFile := filepath.Join(".claude/skills", skill, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				t.Fatalf("expected skill %s to exist before deinit", skill)
			}
		}

		// Set force flag for deinit
		oldForce := deinitForce
		deinitForce = true
		defer func() { deinitForce = oldForce }()

		// Run deinit command
		err = runDeinit(nil, nil)
		if err != nil {
			t.Fatalf("runDeinit failed: %v", err)
		}

		// Verify .rafa directory was removed
		if _, err := os.Stat(".rafa"); !os.IsNotExist(err) {
			t.Error("expected .rafa directory to be removed")
		}

		// Verify all skills were removed
		for _, skill := range skills.RequiredSkills {
			skillDir := filepath.Join(".claude/skills", skill)
			if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
				t.Errorf("expected skill directory %s to be removed", skillDir)
			}
		}

		// Verify .gitignore doesn't contain rafa entries but still has the original entry
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist after deinit: %v", err)
		}
		if strings.Contains(string(content), ".rafa") {
			t.Errorf("expected .gitignore to not have .rafa entries after deinit, got %q", string(content))
		}
		if !strings.Contains(string(content), "node_modules/") {
			t.Errorf("expected .gitignore to still have node_modules entry, got %q", string(content))
		}
	})

	t.Run("deinit completes even when skills removal fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Use integration test skills installer for init
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &integrationTestSkillsInstaller{
				targetDir: targetDir,
			}
		}

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command first
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Now use a failing skills installer for deinit
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &integrationTestSkillsInstaller{
				targetDir:           targetDir,
				shouldFailUninstall: true,
			}
		}

		// Set force flag for deinit
		oldForce := deinitForce
		deinitForce = true
		defer func() { deinitForce = oldForce }()

		// Run deinit command - should succeed despite skills failure
		err = runDeinit(nil, nil)
		if err != nil {
			t.Fatalf("runDeinit should succeed even when skills removal fails: %v", err)
		}

		// Verify .rafa directory was removed (most important)
		if _, err := os.Stat(".rafa"); !os.IsNotExist(err) {
			t.Error("expected .rafa directory to be removed even when skills removal fails")
		}
	})
}

// TestDeinitIntegration_FailsWhenNotInitialized tests deinit fails when .rafa doesn't exist.
func TestDeinitIntegration_FailsWhenNotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Don't create .rafa directory - deinit should fail
	err := runDeinit(nil, nil)
	if err == nil {
		t.Error("expected error when .rafa directory doesn't exist")
	}
}

// TestDeinitIntegration_RemovesAllContent tests that deinit removes nested content.
func TestDeinitIntegration_RemovesAllContent(t *testing.T) {
	// Save originals
	originalCommandFunc := commandFunc
	originalLookPathFunc := lookPathFunc
	originalSkillsFactory := skillsInstallerFactory

	t.Cleanup(func() {
		commandFunc = originalCommandFunc
		lookPathFunc = originalLookPathFunc
		skillsInstallerFactory = originalSkillsFactory
	})

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Mock commands
	commandFunc = func(name string, args ...string) *exec.Cmd {
		if name == "claude" {
			return exec.Command("true")
		}
		return exec.Command(name, args...)
	}
	lookPathFunc = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	skillsInstallerFactory = func(targetDir string) SkillsInstaller {
		return &integrationTestSkillsInstaller{targetDir: targetDir}
	}

	// Initialize git
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Run init
	err := runInit(nil, nil)
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Create additional nested content in .rafa
	nestedDir := filepath.Join(".rafa", "plans", "test-plan")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "plan.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create plan.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "progress.log"), []byte("progress"), 0644); err != nil {
		t.Fatalf("failed to create progress.log: %v", err)
	}

	// Create session files
	if err := os.WriteFile(filepath.Join(".rafa", "sessions", "test-session.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to create session file: %v", err)
	}

	// Set force flag for deinit
	oldForce := deinitForce
	deinitForce = true
	defer func() { deinitForce = oldForce }()

	// Run deinit
	err = runDeinit(nil, nil)
	if err != nil {
		t.Fatalf("runDeinit failed: %v", err)
	}

	// Verify everything is gone
	if _, err := os.Stat(".rafa"); !os.IsNotExist(err) {
		t.Error(".rafa directory should be completely removed")
	}
	if _, err := os.Stat(nestedDir); !os.IsNotExist(err) {
		t.Error("nested plan directory should be removed")
	}
}

// TestDeinitIntegration_PreservesClaudeDirectory tests that deinit only removes skills, not .claude.
func TestDeinitIntegration_PreservesClaudeDirectory(t *testing.T) {
	// Save originals
	originalCommandFunc := commandFunc
	originalLookPathFunc := lookPathFunc
	originalSkillsFactory := skillsInstallerFactory

	t.Cleanup(func() {
		commandFunc = originalCommandFunc
		lookPathFunc = originalLookPathFunc
		skillsInstallerFactory = originalSkillsFactory
	})

	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Mock commands
	commandFunc = func(name string, args ...string) *exec.Cmd {
		if name == "claude" {
			return exec.Command("true")
		}
		return exec.Command(name, args...)
	}
	lookPathFunc = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	skillsInstallerFactory = func(targetDir string) SkillsInstaller {
		return &integrationTestSkillsInstaller{targetDir: targetDir}
	}

	// Initialize git
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Run init
	err := runInit(nil, nil)
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Create a non-skill file in .claude (e.g., settings.json)
	if err := os.WriteFile(filepath.Join(".claude", "settings.json"), []byte(`{"test": true}`), 0644); err != nil {
		t.Fatalf("failed to create settings.json: %v", err)
	}

	// Set force flag for deinit
	oldForce := deinitForce
	deinitForce = true
	defer func() { deinitForce = oldForce }()

	// Run deinit
	err = runDeinit(nil, nil)
	if err != nil {
		t.Fatalf("runDeinit failed: %v", err)
	}

	// Verify .claude directory still exists
	if _, err := os.Stat(".claude"); os.IsNotExist(err) {
		t.Error(".claude directory should still exist (deinit only removes skills)")
	}

	// Verify settings.json still exists
	if _, err := os.Stat(filepath.Join(".claude", "settings.json")); os.IsNotExist(err) {
		t.Error(".claude/settings.json should be preserved")
	}

	// Verify skills are removed
	for _, skill := range skills.RequiredSkills {
		skillDir := filepath.Join(".claude", "skills", skill)
		if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
			t.Errorf("skill %s should be removed", skill)
		}
	}
}
