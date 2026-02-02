package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultMaxLines = 1000

// OutputViewport wraps bubbles/viewport with auto-scroll behavior.
type OutputViewport struct {
	viewport   viewport.Model
	autoScroll bool     // true = scroll to bottom on new content
	lines      []string // buffered lines
	maxLines   int      // ring buffer size
	width      int
	height     int
}

// NewOutputViewport creates a new OutputViewport with the given dimensions.
// maxLines controls the ring buffer size (0 uses default of 1000).
func NewOutputViewport(width, height, maxLines int) OutputViewport {
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}

	vp := viewport.New(width, height)
	vp.SetContent("")

	return OutputViewport{
		viewport:   vp,
		autoScroll: true,
		lines:      make([]string, 0, maxLines),
		maxLines:   maxLines,
		width:      width,
		height:     height,
	}
}

// AddLine appends a line to the buffer, dropping the oldest if buffer is full.
func (o *OutputViewport) AddLine(line string) {
	// Add line to buffer
	if len(o.lines) >= o.maxLines {
		// Drop oldest line (ring buffer behavior)
		o.lines = o.lines[1:]
	}
	o.lines = append(o.lines, line)

	// Update viewport content
	o.viewport.SetContent(strings.Join(o.lines, "\n"))

	// Auto-scroll to bottom if enabled
	if o.autoScroll {
		o.viewport.GotoBottom()
	}
}

// Update handles viewport key events. Scrolling up pauses auto-scroll.
func (o *OutputViewport) Update(msg tea.Msg) (OutputViewport, tea.Cmd) {
	var cmd tea.Cmd

	// Update viewport
	o.viewport, cmd = o.viewport.Update(msg)

	// Check for scroll keys to manage auto-scroll
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k", "pgup", "ctrl+u":
			// User scrolled up, pause auto-scroll
			o.autoScroll = false
		case "down", "j", "pgdown", "ctrl+d":
			// If user scrolled to bottom, re-enable auto-scroll
			if o.viewport.AtBottom() {
				o.autoScroll = true
			}
		case "end", "G":
			// User explicitly went to bottom, re-enable auto-scroll
			o.autoScroll = true
		case "home", "g":
			// User went to top, disable auto-scroll
			o.autoScroll = false
		}
	}

	return *o, cmd
}

// View returns the rendered viewport.
func (o OutputViewport) View() string {
	return o.viewport.View()
}

// SetSize updates the viewport dimensions.
func (o *OutputViewport) SetSize(width, height int) {
	o.width = width
	o.height = height
	o.viewport.Width = width
	o.viewport.Height = height
}

// AutoScroll returns whether auto-scroll is currently enabled.
func (o OutputViewport) AutoScroll() bool {
	return o.autoScroll
}

// SetAutoScroll enables or disables auto-scroll.
func (o *OutputViewport) SetAutoScroll(enabled bool) {
	o.autoScroll = enabled
	if enabled {
		o.viewport.GotoBottom()
	}
}

// LineCount returns the number of lines currently in the buffer.
func (o OutputViewport) LineCount() int {
	return len(o.lines)
}

// Clear removes all lines from the buffer.
func (o *OutputViewport) Clear() {
	o.lines = o.lines[:0]
	o.viewport.SetContent("")
	o.autoScroll = true
}

// SetContent replaces the viewport content with the given text.
// Text is word-wrapped at the viewport width before being set.
// Use this for streaming text that should be displayed as continuous prose.
func (o *OutputViewport) SetContent(content string) {
	// Word-wrap the content at viewport width
	// lipgloss.NewStyle().Width(w).Render() performs word-wrapping
	wrapped := lipgloss.NewStyle().Width(o.width).Render(content)

	// Store the wrapped content (now has newlines from wrapping)
	o.lines = strings.Split(wrapped, "\n")
	o.viewport.SetContent(wrapped)

	if o.autoScroll {
		o.viewport.GotoBottom()
	}
}
