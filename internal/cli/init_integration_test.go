package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablasso/rafa/internal/skills"
)

// TestInitIntegration tests the end-to-end init flow with actual skills installation.
// This test uses the real skills installer but with a mock HTTP client to avoid
// network dependencies while still testing the full integration.
func TestInitIntegration(t *testing.T) {
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

	t.Run("end-to-end init with skills installation", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Use a mock skills installer that creates real directory structure
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			// Create an installer that will create the expected structure
			return &integrationTestSkillsInstaller{
				targetDir: targetDir,
			}
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

		// Verify complete directory structure
		expectedDirs := []string{
			".rafa",
			".rafa/plans",
			".rafa/sessions",
			".claude/skills",
		}
		for _, dir := range expectedDirs {
			info, err := os.Stat(dir)
			if err != nil {
				t.Errorf("expected directory %s to exist, got error: %v", dir, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("expected %s to be a directory", dir)
			}
		}

		// Verify all required skills are installed with SKILL.md
		for _, skill := range skills.RequiredSkills {
			skillFile := filepath.Join(".claude/skills", skill, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				t.Errorf("expected skill %s to have SKILL.md at %s", skill, skillFile)
			}
		}

		// Verify .gitignore contains both required entries
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist, got error: %v", err)
		}
		gitignoreStr := string(content)
		if !strings.Contains(gitignoreStr, ".rafa/**/*.lock") {
			t.Errorf("expected .gitignore to contain lock pattern, got %q", gitignoreStr)
		}
		if !strings.Contains(gitignoreStr, ".rafa/sessions/") {
			t.Errorf("expected .gitignore to contain sessions pattern, got %q", gitignoreStr)
		}

		// Verify skills installer correctly used the .claude/skills path
		skillsDir := filepath.Join(tmpDir, ".claude", "skills")
		info, err := os.Stat(skillsDir)
		if err != nil {
			t.Errorf("expected .claude/skills directory to exist, got error: %v", err)
		} else if !info.IsDir() {
			t.Errorf("expected .claude/skills to be a directory")
		}
	})

	t.Run("end-to-end init with failed skills installation cleans up everything", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Use a mock skills installer that fails
		skillsInstallerFactory = func(targetDir string) SkillsInstaller {
			return &integrationTestSkillsInstaller{
				targetDir:  targetDir,
				shouldFail: true,
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
			t.Fatal("expected error when skills installation fails, got nil")
		}

		// Verify no partial state remains
		notExpected := []string{
			".rafa",
			".rafa/plans",
			".rafa/sessions",
		}
		for _, path := range notExpected {
			if _, err := os.Stat(path); err == nil {
				t.Errorf("expected %s to not exist after failed init", path)
			}
		}

		// Verify .gitignore doesn't have rafa entries
		content, err := os.ReadFile(".gitignore")
		if err == nil && strings.Contains(string(content), ".rafa") {
			t.Errorf("expected .gitignore to not have .rafa entries after failed init, got %q", string(content))
		}
	})
}

// integrationTestSkillsInstaller simulates the real installer for integration tests.
type integrationTestSkillsInstaller struct {
	targetDir  string
	shouldFail bool
}

func (i *integrationTestSkillsInstaller) Install() error {
	if i.shouldFail {
		return os.ErrNotExist // Simulate skills repo unavailable
	}

	// Create all required skills with SKILL.md files
	for _, skill := range skills.RequiredSkills {
		skillDir := filepath.Join(i.targetDir, skill)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return err
		}
		skillFile := filepath.Join(skillDir, "SKILL.md")
		content := "# " + skill + "\n\nSkill description for " + skill
		if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (i *integrationTestSkillsInstaller) Uninstall() error {
	// Remove all skills
	for _, skill := range skills.RequiredSkills {
		skillDir := filepath.Join(i.targetDir, skill)
		os.RemoveAll(skillDir)
	}
	return nil
}
