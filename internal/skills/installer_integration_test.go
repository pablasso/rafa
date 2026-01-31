package skills

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestInstallFromGitHub_Integration tests real HTTP download of skills from GitHub.
// This test requires network access and is skipped by default unless
// RAFA_RUN_NETWORK_TESTS=1 is set. This allows testing against the real
// GitHub repository without affecting CI builds.
func TestInstallFromGitHub_Integration(t *testing.T) {
	// Skip unless explicitly enabled - network tests should be opt-in
	if os.Getenv("RAFA_RUN_NETWORK_TESTS") != "1" {
		t.Skip("Skipping network test (set RAFA_RUN_NETWORK_TESTS=1 to enable)")
	}

	tmpDir := t.TempDir()

	installer := NewInstaller(tmpDir)
	// Use the real HTTP client (default)
	installer.SetHTTPClient(http.DefaultClient)

	err := installer.Install()
	if err != nil {
		t.Fatalf("Install() from GitHub failed: %v", err)
	}

	// Verify all 5 required skills are installed
	if len(RequiredSkills) != 5 {
		t.Errorf("expected 5 required skills, got %d", len(RequiredSkills))
	}

	for _, skill := range RequiredSkills {
		skillDir := filepath.Join(tmpDir, skill)

		// Check directory exists
		info, err := os.Stat(skillDir)
		if os.IsNotExist(err) {
			t.Errorf("skill directory %q was not created", skill)
			continue
		}
		if err != nil {
			t.Errorf("error checking skill directory %q: %v", skill, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("skill path %q is not a directory", skill)
			continue
		}

		// Check SKILL.md exists
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			t.Errorf("SKILL.md not found for skill %q", skill)
		}
	}

	// Verify IsInstalled returns true
	if !installer.IsInstalled() {
		t.Error("IsInstalled() returned false after successful Install()")
	}

	// Verify specific skills by name
	expectedSkills := []string{
		"prd",
		"prd-review",
		"technical-design",
		"technical-design-review",
		"code-review",
	}

	for _, skill := range expectedSkills {
		found := false
		for _, rs := range RequiredSkills {
			if rs == skill {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected skill %q not in RequiredSkills", skill)
		}
	}
}

// TestInstallFromGitHub_NetworkUnavailable tests behavior when network is unavailable.
// This is a mock test that simulates network failure without requiring real network access.
func TestInstallFromGitHub_NetworkUnavailable(t *testing.T) {
	tmpDir := t.TempDir()

	installer := NewInstaller(tmpDir)

	// Use a mock HTTP client that fails
	installer.SetHTTPClient(&failingHTTPClient{})

	err := installer.Install()
	if err == nil {
		t.Error("expected error when network is unavailable")
	}

	// Verify no partial state remains
	for _, skill := range RequiredSkills {
		skillDir := filepath.Join(tmpDir, skill)
		if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
			t.Errorf("skill directory %q should not exist after network failure", skill)
		}
	}
}

// failingHTTPClient is a test helper that always returns network errors.
type failingHTTPClient struct{}

func (f *failingHTTPClient) Get(url string) (*http.Response, error) {
	return nil, &networkError{msg: "simulated network failure"}
}

type networkError struct {
	msg string
}

func (e *networkError) Error() string {
	return e.msg
}
