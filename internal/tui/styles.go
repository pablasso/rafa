package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4") // Purple accent
	secondaryColor = lipgloss.Color("#6C6C6C") // Gray for secondary text
	successColor   = lipgloss.Color("#73F59F") // Green for success
	errorColor     = lipgloss.Color("#FF6B6B") // Red for errors

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
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	// BoxStyle for panel borders
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2)

	// SuccessStyle for success messages
	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)
)
