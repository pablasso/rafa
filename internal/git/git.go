package git

import (
	"os/exec"
	"strings"
)

// Status represents the git workspace status.
type Status struct {
	Clean bool
	Files []string
}

// GetStatus returns the git workspace status for the given directory.
// If dir is empty, uses the current working directory.
func GetStatus(dir string) (*Status, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	if dir != "" {
		cmd.Dir = dir
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// git status --porcelain format: XY filename
		// XY is the status (2 chars), followed by a space and filename
		// e.g., "?? file.txt", " M file.txt", "A  file.txt"
		if len(line) > 3 {
			files = append(files, line[3:])
		} else {
			// Unexpected format, include the whole line as the filename
			// to avoid silently dropping entries
			files = append(files, strings.TrimSpace(line))
		}
	}

	return &Status{
		Clean: len(files) == 0,
		Files: files,
	}, nil
}

// IsClean returns true if the git workspace has no uncommitted changes.
// It checks both staged and unstaged changes, as well as untracked files.
// If dir is empty, uses the current working directory.
func IsClean(dir string) (bool, error) {
	status, err := GetStatus(dir)
	if err != nil {
		return false, err
	}
	return status.Clean, nil
}

// GetDirtyFiles returns a list of files with uncommitted changes.
// This includes modified, staged, and untracked files.
// If dir is empty, uses the current working directory.
func GetDirtyFiles(dir string) ([]string, error) {
	status, err := GetStatus(dir)
	if err != nil {
		return nil, err
	}
	return status.Files, nil
}
