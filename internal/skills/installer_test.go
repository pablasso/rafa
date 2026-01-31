package skills

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	return m.response, m.err
}

// createTestTarball creates a gzipped tarball with the given files.
// Files is a map from path (e.g., "prd/SKILL.md") to content.
// The tarball will have a root prefix like "pablasso-skills-abc123/".
func createTestTarball(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	rootPrefix := "pablasso-skills-abc123/"

	// Add root directory
	if err := tw.WriteHeader(&tar.Header{
		Name:     rootPrefix,
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return nil, err
	}

	// Track directories we've already added
	addedDirs := make(map[string]bool)

	for path, content := range files {
		// Add parent directories
		dir := filepath.Dir(path)
		if dir != "." && !addedDirs[dir] {
			if err := tw.WriteHeader(&tar.Header{
				Name:     rootPrefix + dir + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}); err != nil {
				return nil, err
			}
			addedDirs[dir] = true
		}

		// Add file
		if err := tw.WriteHeader(&tar.Header{
			Name:     rootPrefix + path,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return nil, err
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestNewInstaller(t *testing.T) {
	t.Run("creates installer with target directory", func(t *testing.T) {
		installer := NewInstaller("/path/to/skills")
		if installer.targetDir != "/path/to/skills" {
			t.Errorf("targetDir = %q, want %q", installer.targetDir, "/path/to/skills")
		}
		if installer.httpClient == nil {
			t.Error("httpClient should not be nil")
		}
		if installer.skillsURL != DefaultSkillsURL {
			t.Errorf("skillsURL = %q, want %q", installer.skillsURL, DefaultSkillsURL)
		}
	})
}

func TestInstaller_Install(t *testing.T) {
	t.Run("downloads and extracts all required skills", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a tarball with all required skills
		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd/templates/template.md":        "PRD template content",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			"code-review/SKILL.md":             "# Code Review Skill",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		// Create mock HTTP client
		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// Verify all required skills are installed
		for _, skill := range RequiredSkills {
			skillFile := filepath.Join(tmpDir, skill, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				t.Errorf("skill %q SKILL.md not installed", skill)
			}
		}

		// Verify additional files within skills are also extracted
		templateFile := filepath.Join(tmpDir, "prd", "templates", "template.md")
		if _, err := os.Stat(templateFile); os.IsNotExist(err) {
			t.Error("prd/templates/template.md should be extracted")
		}
	})

	t.Run("returns error on HTTP failure", func(t *testing.T) {
		tmpDir := t.TempDir()

		client := &mockHTTPClient{
			err: errors.New("network error"),
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err := installer.Install()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to download skills") {
			t.Errorf("error should mention download failure, got: %v", err)
		}
	})

	t.Run("returns error on non-200 HTTP status", func(t *testing.T) {
		tmpDir := t.TempDir()

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("not found")),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err := installer.Install()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "HTTP 404") {
			t.Errorf("error should mention HTTP 404, got: %v", err)
		}
	})

	t.Run("returns error on invalid gzip", func(t *testing.T) {
		tmpDir := t.TempDir()

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("not a gzip file")),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err := installer.Install()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to extract skills") {
			t.Errorf("error should mention extraction failure, got: %v", err)
		}
	})
}

func TestInstaller_Install_FiltersNonSkillDirectories(t *testing.T) {
	t.Run("does not copy .claude directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			"code-review/SKILL.md":             "# Code Review Skill",
			".claude/settings.json":            "{\"setting\": true}",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// .claude should NOT be extracted
		claudeDir := filepath.Join(tmpDir, ".claude")
		if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
			t.Error(".claude directory should not be extracted")
		}
	})

	t.Run("does not copy README.md at root", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			"code-review/SKILL.md":             "# Code Review Skill",
			"README.md":                        "# Skills Repository",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// README.md should NOT be extracted (it's a root-level file, not in a skill dir)
		// The way our filter works, "README.md" has no skill name prefix, so it won't match
		readmeFile := filepath.Join(tmpDir, "README.md")
		if _, err := os.Stat(readmeFile); !os.IsNotExist(err) {
			t.Error("README.md should not be extracted")
		}
	})

	t.Run("does not copy CLAUDE.md at root", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			"code-review/SKILL.md":             "# Code Review Skill",
			"CLAUDE.md":                        "# Claude instructions",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		claudeFile := filepath.Join(tmpDir, "CLAUDE.md")
		if _, err := os.Stat(claudeFile); !os.IsNotExist(err) {
			t.Error("CLAUDE.md should not be extracted")
		}
	})

	t.Run("does not copy unrecognized skill directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			"code-review/SKILL.md":             "# Code Review Skill",
			"some-other-skill/SKILL.md":        "# Some Other Skill",
			"random-directory/file.txt":        "random content",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// Unrecognized directories should NOT be extracted
		otherSkillDir := filepath.Join(tmpDir, "some-other-skill")
		if _, err := os.Stat(otherSkillDir); !os.IsNotExist(err) {
			t.Error("some-other-skill directory should not be extracted")
		}

		randomDir := filepath.Join(tmpDir, "random-directory")
		if _, err := os.Stat(randomDir); !os.IsNotExist(err) {
			t.Error("random-directory should not be extracted")
		}
	})
}

func TestInstaller_Verify(t *testing.T) {
	t.Run("fails when SKILL.md is missing from a skill", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create tarball missing one SKILL.md
		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			// code-review/SKILL.md is missing!
			"code-review/README.md": "# Code Review - missing SKILL.md",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err == nil {
			t.Fatal("expected error when SKILL.md is missing")
		}
		if !strings.Contains(err.Error(), "code-review") {
			t.Errorf("error should mention code-review, got: %v", err)
		}
		if !strings.Contains(err.Error(), "SKILL.md") {
			t.Errorf("error should mention SKILL.md, got: %v", err)
		}
	})

	t.Run("fails when entire skill directory is missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create tarball missing an entire skill directory
		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			// code-review directory is entirely missing!
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err == nil {
			t.Fatal("expected error when skill directory is missing")
		}
		if !strings.Contains(err.Error(), "code-review") {
			t.Errorf("error should mention code-review, got: %v", err)
		}
	})
}

func TestInstaller_Install_CleansUpOnError(t *testing.T) {
	t.Run("cleans up partial extraction on verification failure", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create tarball with incomplete skills (missing code-review SKILL.md)
		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			// code-review/SKILL.md is missing - will trigger verification failure
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// All partial installations should be cleaned up
		for _, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
				t.Errorf("skill directory %q should be cleaned up after failure", skill)
			}
		}
	})

	t.Run("cleans up partial extraction on tarball error", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a truncated/corrupted tarball
		// Start with a valid tarball but truncate it
		files := map[string]string{
			"prd/SKILL.md": "# PRD Skill",
		}
		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		// Truncate to simulate a download error or corrupted file
		truncated := tarball[:len(tarball)/2]

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(truncated)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err == nil {
			t.Fatal("expected error from truncated tarball")
		}

		// Partial installations should be cleaned up
		for _, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
				t.Errorf("skill directory %q should be cleaned up after extraction failure", skill)
			}
		}
	})
}

func TestInstaller_IsInstalled(t *testing.T) {
	t.Run("returns true when all skills have SKILL.md", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create all required skill directories with SKILL.md
		for _, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatalf("failed to create skill dir: %v", err)
			}
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if err := os.WriteFile(skillFile, []byte("# Skill"), 0644); err != nil {
				t.Fatalf("failed to create SKILL.md: %v", err)
			}
		}

		installer := NewInstaller(tmpDir)
		if !installer.IsInstalled() {
			t.Error("IsInstalled() = false, want true when all skills have SKILL.md")
		}
	})

	t.Run("returns false when one skill is missing SKILL.md", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create all skills except one is missing SKILL.md
		for i, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatalf("failed to create skill dir: %v", err)
			}
			if i < len(RequiredSkills)-1 { // Skip last one
				skillFile := filepath.Join(skillDir, "SKILL.md")
				if err := os.WriteFile(skillFile, []byte("# Skill"), 0644); err != nil {
					t.Fatalf("failed to create SKILL.md: %v", err)
				}
			}
		}

		installer := NewInstaller(tmpDir)
		if installer.IsInstalled() {
			t.Error("IsInstalled() = true, want false when one skill is missing SKILL.md")
		}
	})

	t.Run("returns false when no skills are installed", func(t *testing.T) {
		tmpDir := t.TempDir()

		installer := NewInstaller(tmpDir)
		if installer.IsInstalled() {
			t.Error("IsInstalled() = true, want false when no skills are installed")
		}
	})

	t.Run("returns false when skill directory exists but SKILL.md is missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create all skill directories but only some have SKILL.md
		for i, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatalf("failed to create skill dir: %v", err)
			}
			if i%2 == 0 { // Only even-indexed skills get SKILL.md
				skillFile := filepath.Join(skillDir, "SKILL.md")
				if err := os.WriteFile(skillFile, []byte("# Skill"), 0644); err != nil {
					t.Fatalf("failed to create SKILL.md: %v", err)
				}
			} else {
				// Create a different file instead
				otherFile := filepath.Join(skillDir, "README.md")
				if err := os.WriteFile(otherFile, []byte("# Readme"), 0644); err != nil {
					t.Fatalf("failed to create README.md: %v", err)
				}
			}
		}

		installer := NewInstaller(tmpDir)
		if installer.IsInstalled() {
			t.Error("IsInstalled() = true, want false when some skills are missing SKILL.md")
		}
	})
}

func TestInstaller_Uninstall(t *testing.T) {
	t.Run("removes all required skill directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create all skill directories with some files
		for _, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatalf("failed to create skill dir: %v", err)
			}
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if err := os.WriteFile(skillFile, []byte("# Skill"), 0644); err != nil {
				t.Fatalf("failed to create SKILL.md: %v", err)
			}
		}

		installer := NewInstaller(tmpDir)
		err := installer.Uninstall()
		if err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}

		// All skill directories should be removed
		for _, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
				t.Errorf("skill directory %q should be removed", skill)
			}
		}
	})

	t.Run("preserves non-skill directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create skill directories
		for _, skill := range RequiredSkills {
			skillDir := filepath.Join(tmpDir, skill)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatalf("failed to create skill dir: %v", err)
			}
		}

		// Create a non-skill directory
		otherDir := filepath.Join(tmpDir, "some-other-directory")
		if err := os.MkdirAll(otherDir, 0755); err != nil {
			t.Fatalf("failed to create other dir: %v", err)
		}
		otherFile := filepath.Join(otherDir, "file.txt")
		if err := os.WriteFile(otherFile, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		installer := NewInstaller(tmpDir)
		err := installer.Uninstall()
		if err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}

		// Non-skill directory should be preserved
		if _, err := os.Stat(otherDir); os.IsNotExist(err) {
			t.Error("non-skill directory should be preserved")
		}
		if _, err := os.Stat(otherFile); os.IsNotExist(err) {
			t.Error("file in non-skill directory should be preserved")
		}
	})

	t.Run("is idempotent - no error when skills already removed", func(t *testing.T) {
		tmpDir := t.TempDir()

		installer := NewInstaller(tmpDir)

		// Uninstall on empty directory
		err := installer.Uninstall()
		if err != nil {
			t.Fatalf("first Uninstall() error = %v", err)
		}

		// Uninstall again
		err = installer.Uninstall()
		if err != nil {
			t.Fatalf("second Uninstall() error = %v", err)
		}
	})

	t.Run("removes nested files within skill directories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create nested structure in a skill
		prdDir := filepath.Join(tmpDir, "prd")
		nestedDir := filepath.Join(prdDir, "templates", "nested")
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			t.Fatalf("failed to create nested dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(prdDir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
			t.Fatalf("failed to create SKILL.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(nestedDir, "template.md"), []byte("template"), 0644); err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		installer := NewInstaller(tmpDir)
		err := installer.Uninstall()
		if err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}

		// Entire prd directory should be removed
		if _, err := os.Stat(prdDir); !os.IsNotExist(err) {
			t.Error("prd directory should be removed including nested content")
		}
	})
}

func TestInstaller_ExtractTarball_StripsPrefixCorrectly(t *testing.T) {
	t.Run("strips GitHub root directory prefix", func(t *testing.T) {
		tmpDir := t.TempDir()

		files := map[string]string{
			"prd/SKILL.md":                     "# PRD Skill",
			"prd-review/SKILL.md":              "# PRD Review Skill",
			"technical-design/SKILL.md":        "# Technical Design Skill",
			"technical-design-review/SKILL.md": "# Technical Design Review Skill",
			"code-review/SKILL.md":             "# Code Review Skill",
		}

		tarball, err := createTestTarball(files)
		if err != nil {
			t.Fatalf("failed to create test tarball: %v", err)
		}

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(tarball)),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err = installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// Files should be at skill/SKILL.md, not pablasso-skills-abc123/skill/SKILL.md
		prdSkill := filepath.Join(tmpDir, "prd", "SKILL.md")
		if _, err := os.Stat(prdSkill); os.IsNotExist(err) {
			t.Error("prd/SKILL.md should exist (root prefix stripped)")
		}

		// Root prefix directory should not exist
		rootPrefixDir := filepath.Join(tmpDir, "pablasso-skills-abc123")
		if _, err := os.Stat(rootPrefixDir); !os.IsNotExist(err) {
			t.Error("root prefix directory should not be created")
		}
	})
}

func TestRequiredSkills(t *testing.T) {
	t.Run("contains all expected skills", func(t *testing.T) {
		expected := []string{
			"prd",
			"prd-review",
			"technical-design",
			"technical-design-review",
			"code-review",
		}

		if len(RequiredSkills) != len(expected) {
			t.Errorf("RequiredSkills has %d items, want %d", len(RequiredSkills), len(expected))
		}

		for _, skill := range expected {
			found := false
			for _, rs := range RequiredSkills {
				if rs == skill {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("RequiredSkills should contain %q", skill)
			}
		}
	})
}

func TestInstaller_ExtractTarball_Security(t *testing.T) {
	t.Run("rejects paths with parent directory traversal", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a tarball with path traversal attempt
		// We need to manually build this tarball since our helper won't allow ".." in paths
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gzw)

		rootPrefix := "pablasso-skills-abc123/"

		// Add root directory
		tw.WriteHeader(&tar.Header{
			Name:     rootPrefix,
			Mode:     0755,
			Typeflag: tar.TypeDir,
		})

		// Add a valid skill for the install to succeed
		for _, skill := range RequiredSkills {
			tw.WriteHeader(&tar.Header{
				Name:     rootPrefix + skill + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			})
			content := "# " + skill + " Skill"
			tw.WriteHeader(&tar.Header{
				Name:     rootPrefix + skill + "/SKILL.md",
				Mode:     0644,
				Size:     int64(len(content)),
				Typeflag: tar.TypeReg,
			})
			tw.Write([]byte(content))
		}

		// Add a malicious path traversal file
		maliciousContent := "malicious content"
		tw.WriteHeader(&tar.Header{
			Name:     rootPrefix + "prd/../../../etc/malicious.txt",
			Mode:     0644,
			Size:     int64(len(maliciousContent)),
			Typeflag: tar.TypeReg,
		})
		tw.Write([]byte(maliciousContent))

		tw.Close()
		gzw.Close()

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&buf),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err := installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// The malicious file should NOT be created
		// Check that nothing was created outside the target directory
		maliciousPath := filepath.Join(tmpDir, "..", "..", "..", "etc", "malicious.txt")
		if _, err := os.Stat(maliciousPath); !os.IsNotExist(err) {
			t.Error("path traversal file should not be extracted")
		}

		// Also check it wasn't placed in the skills directory with ".." in the name
		badPath := filepath.Join(tmpDir, "prd", "..", "..", "..", "etc", "malicious.txt")
		if _, err := os.Stat(badPath); !os.IsNotExist(err) {
			t.Error("path traversal file should not be extracted")
		}

		// Valid skills should still be installed
		for _, skill := range RequiredSkills {
			skillFile := filepath.Join(tmpDir, skill, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				t.Errorf("valid skill %q should still be installed", skill)
			}
		}
	})

	t.Run("ignores symlinks in tarball", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a tarball with a symlink
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gzw)

		rootPrefix := "pablasso-skills-abc123/"

		// Add root directory
		tw.WriteHeader(&tar.Header{
			Name:     rootPrefix,
			Mode:     0755,
			Typeflag: tar.TypeDir,
		})

		// Add valid skills
		for _, skill := range RequiredSkills {
			tw.WriteHeader(&tar.Header{
				Name:     rootPrefix + skill + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			})
			content := "# " + skill + " Skill"
			tw.WriteHeader(&tar.Header{
				Name:     rootPrefix + skill + "/SKILL.md",
				Mode:     0644,
				Size:     int64(len(content)),
				Typeflag: tar.TypeReg,
			})
			tw.Write([]byte(content))
		}

		// Add a symlink (should be ignored)
		tw.WriteHeader(&tar.Header{
			Name:     rootPrefix + "prd/link-to-etc",
			Mode:     0777,
			Typeflag: tar.TypeSymlink,
			Linkname: "/etc/passwd",
		})

		tw.Close()
		gzw.Close()

		client := &mockHTTPClient{
			response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&buf),
			},
		}

		installer := NewInstaller(tmpDir)
		installer.SetHTTPClient(client)

		err := installer.Install()
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		// The symlink should NOT be created
		symlinkPath := filepath.Join(tmpDir, "prd", "link-to-etc")
		if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
			t.Error("symlink should not be extracted")
		}

		// Valid skills should still be installed
		if !installer.IsInstalled() {
			t.Error("valid skills should still be installed despite ignored symlink")
		}
	})
}
