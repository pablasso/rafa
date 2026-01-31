// Package styles defines shared lipgloss styles for the TUI.
package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#5FAFAF") // Teal accent
	secondaryColor = lipgloss.Color("#666666") // Gray for secondary text
	successColor   = lipgloss.Color("#87AF87") // Muted sage for success
	errorColor     = lipgloss.Color("#AF5F5F") // Muted terracotta for errors

	// TitleStyle for headers
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// SubtleStyle for hints/help text
	SubtleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// SelectedStyle for selected items in lists
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	// StatusBarStyle for bottom status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// BoxStyle for panel borders
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)

	// SuccessStyle for success messages
	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)
)
