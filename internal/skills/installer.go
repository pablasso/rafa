package skills

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultSkillsURL is the URL to download skills from.
	// GitHub API provides tarballs at /repos/{owner}/{repo}/tarball/{ref}
	DefaultSkillsURL = "https://api.github.com/repos/pablasso/skills/tarball/main"
)

// RequiredSkills lists the skills that must be installed.
// Each skill directory must contain a SKILL.md file.
var RequiredSkills = []string{
	"prd",
	"prd-review",
	"technical-design",
	"technical-design-review",
	"code-review",
}

// HTTPClient is an interface for HTTP operations to allow mocking in tests.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// Installer handles downloading and installing skills.
type Installer struct {
	targetDir  string     // .claude/skills/
	httpClient HTTPClient // HTTP client for downloading
	skillsURL  string     // URL to download skills from
}

// NewInstaller creates an installer targeting the given directory.
func NewInstaller(targetDir string) *Installer {
	return &Installer{
		targetDir:  targetDir,
		httpClient: http.DefaultClient,
		skillsURL:  DefaultSkillsURL,
	}
}

// SetHTTPClient sets a custom HTTP client (useful for testing).
func (i *Installer) SetHTTPClient(client HTTPClient) {
	i.httpClient = client
}

// SetSkillsURL sets a custom URL for downloading skills (useful for testing).
func (i *Installer) SetSkillsURL(url string) {
	i.skillsURL = url
}

// Install downloads and extracts skills from GitHub.
func (i *Installer) Install() error {
	// Create target directory
	if err := os.MkdirAll(i.targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	// Download tarball
	resp, err := i.httpClient.Get(i.skillsURL)
	if err != nil {
		return fmt.Errorf("failed to download skills: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download skills: HTTP %d", resp.StatusCode)
	}

	// Extract tarball
	if err := i.extractTarball(resp.Body); err != nil {
		// Clean up partial installation
		i.Uninstall()
		return fmt.Errorf("failed to extract skills: %w", err)
	}

	// Verify required skills are present with SKILL.md files
	if err := i.verify(); err != nil {
		// Clean up partial installation
		i.Uninstall()
		return err
	}

	return nil
}

// extractTarball extracts skills from the GitHub tarball.
// Only extracts directories that match RequiredSkills.
// GitHub tarballs have a root directory prefix that is stripped.
func (i *Installer) extractTarball(r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// GitHub tarballs have a root directory like "pablasso-skills-abc123/"
	// We need to strip this prefix
	var rootPrefix string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Detect root prefix from first entry
		if rootPrefix == "" {
			parts := strings.SplitN(header.Name, "/", 2)
			if len(parts) > 0 {
				rootPrefix = parts[0] + "/"
			}
		}

		// Strip root prefix
		relPath := strings.TrimPrefix(header.Name, rootPrefix)
		if relPath == "" {
			continue
		}

		// Parse path: first component is potential skill name
		parts := strings.SplitN(relPath, "/", 2)
		skillName := parts[0]

		// Only extract files inside required skill directories
		if !i.isRequiredSkill(skillName) {
			continue
		}

		// Security: reject paths with parent directory traversal
		if strings.Contains(relPath, "..") {
			continue
		}

		targetPath := filepath.Join(i.targetDir, relPath)

		// Security: verify the target is within the target directory
		// This catches edge cases that simple ".." checking might miss
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(i.targetDir)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}
			f, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}
			// Use a limited reader to prevent decompression bombs
			limited := io.LimitReader(tr, 10*1024*1024) // 10MB max per file
			if _, err := io.Copy(f, limited); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			f.Close()
		}
	}

	return nil
}

// isRequiredSkill checks if a name matches a required skill.
func (i *Installer) isRequiredSkill(name string) bool {
	for _, skill := range RequiredSkills {
		if name == skill {
			return true
		}
	}
	return false
}

// verify checks that all required skills are installed with SKILL.md files.
func (i *Installer) verify() error {
	for _, skill := range RequiredSkills {
		skillFile := filepath.Join(i.targetDir, skill, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			return fmt.Errorf("required skill %q missing SKILL.md file", skill)
		}
	}
	return nil
}

// IsInstalled checks if all required skills are installed with SKILL.md files.
func (i *Installer) IsInstalled() bool {
	for _, skill := range RequiredSkills {
		skillFile := filepath.Join(i.targetDir, skill, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// Uninstall removes installed skills (only the required skill directories).
func (i *Installer) Uninstall() error {
	for _, skill := range RequiredSkills {
		skillDir := filepath.Join(i.targetDir, skill)
		if err := os.RemoveAll(skillDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove skill %q: %w", skill, err)
		}
	}
	return nil
}
