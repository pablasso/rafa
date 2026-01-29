package components

import (
	"strings"

	"github.com/pablasso/rafa/internal/tui/styles"
)

// StatusBar renders a bottom help bar showing contextual help items.
type StatusBar struct{}

// NewStatusBar creates a new StatusBar instance.
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// Render returns the status bar string for the given width and items.
// Items are joined with " • " separator and padded to fill the width.
func (s StatusBar) Render(width int, items []string) string {
	if len(items) == 0 {
		return styles.StatusBarStyle.Width(width).Render("")
	}

	content := strings.Join(items, " • ")

	return styles.StatusBarStyle.Width(width).Render(content)
}
