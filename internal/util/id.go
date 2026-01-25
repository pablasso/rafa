package util

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode"
)

const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateShortID returns a 6-character alphanumeric string using cryptographic randomness.
func GenerateShortID() (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i := range bytes {
		bytes[i] = alphanumeric[int(bytes[i])%len(alphanumeric)]
	}

	return string(bytes), nil
}

// GenerateTaskID returns a task ID in the format t01, t02, ..., t99, t100, etc.
func GenerateTaskID(index int) string {
	return fmt.Sprintf("t%02d", index+1)
}

// toKebabCase converts a string to kebab-case.
// It lowercases the string, replaces spaces and underscores with hyphens,
// removes non-alphanumeric characters (except hyphens), collapses multiple
// consecutive hyphens, and trims leading/trailing hyphens.
func toKebabCase(s string) string {
	var result strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(unicode.ToLower(r))
		} else if r == ' ' || r == '_' || r == '-' {
			result.WriteRune('-')
		}
		// Other characters are dropped
	}

	// Collapse multiple consecutive hyphens
	str := result.String()
	for strings.Contains(str, "--") {
		str = strings.ReplaceAll(str, "--", "-")
	}

	// Trim leading/trailing hyphens
	str = strings.Trim(str, "-")

	return str
}
