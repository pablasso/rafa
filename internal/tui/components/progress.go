package components

import (
	"fmt"
	"strings"
)

const (
	filledChar = "■"
	emptyChar  = "□"
)

// Progress renders a progress bar like: ■■■■□□□□ 50%
type Progress struct {
	Current int
	Total   int
	Width   int // character width of the bar portion
}

// NewProgress creates a new Progress instance.
func NewProgress(current, total, width int) Progress {
	return Progress{
		Current: current,
		Total:   total,
		Width:   width,
	}
}

// View returns the rendered progress bar string.
func (p Progress) View() string {
	if p.Total <= 0 || p.Width <= 0 {
		return ""
	}

	// Clamp current to valid range
	current := p.Current
	if current < 0 {
		current = 0
	}
	if current > p.Total {
		current = p.Total
	}

	// Calculate percentage
	percent := (current * 100) / p.Total

	// Calculate filled portion
	filled := (current * p.Width) / p.Total

	// Build the bar
	bar := strings.Repeat(filledChar, filled) + strings.Repeat(emptyChar, p.Width-filled)

	return fmt.Sprintf("%s %d%%", bar, percent)
}
