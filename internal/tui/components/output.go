package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const defaultMaxLines = 1000

type outputMode int

const (
	outputModeLines outputMode = iota
	outputModeContent
)

// OutputViewport wraps bubbles/viewport with auto-scroll behavior.
type OutputViewport struct {
	viewport   viewport.Model
	autoScroll bool     // true = scroll to bottom on new content
	rawLines   []string // buffered raw lines (unwrapped)
	rawContent string   // raw content when using SetContent
	lines      []string // buffered lines
	maxLines   int      // ring buffer size
	width      int
	height     int
	mode       outputMode
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
		rawLines:   make([]string, 0, maxLines),
		lines:      make([]string, 0, maxLines),
		maxLines:   maxLines,
		width:      width,
		height:     height,
		mode:       outputModeLines,
	}
}

// AddLine appends a line to the buffer, dropping the oldest if buffer is full.
func (o *OutputViewport) AddLine(line string) {
	o.mode = outputModeLines
	o.rawContent = ""

	if len(o.rawLines) >= o.maxLines {
		o.rawLines = o.rawLines[1:]
	}
	o.rawLines = append(o.rawLines, line)

	wrapped := line
	if o.width > 0 {
		wrapped = ansi.Wrap(line, o.width, "/")
	}
	for _, l := range strings.Split(wrapped, "\n") {
		// Add line to buffer
		if len(o.lines) >= o.maxLines {
			// Drop oldest line (ring buffer behavior)
			o.lines = o.lines[1:]
		}
		o.lines = append(o.lines, l)
	}

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
	oldWidth := o.width
	oldHeight := o.height
	if oldWidth == width && oldHeight == height {
		return
	}

	wasAtBottom := o.viewport.AtBottom()
	scrollPercent := o.viewport.ScrollPercent()

	o.width = width
	o.height = height
	o.viewport.Width = width
	o.viewport.Height = height

	if width != oldWidth {
		o.rewrap()
	} else {
		// Clamp y-offset after height change to avoid showing blank space.
		o.viewport.SetYOffset(o.viewport.YOffset)
	}

	if o.autoScroll || wasAtBottom {
		o.viewport.GotoBottom()
		return
	}

	// Only attempt to preserve relative scroll position when width changes,
	// because rewrapping changes the total number of lines.
	if width != oldWidth {
		newMaxYOffset := max(0, len(o.lines)-o.viewport.Height+o.viewport.Style.GetVerticalFrameSize())
		o.viewport.SetYOffset(int(scrollPercent * float64(newMaxYOffset)))
	}
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
	o.rawLines = o.rawLines[:0]
	o.rawContent = ""
	o.mode = outputModeLines
	o.viewport.SetContent("")
	o.autoScroll = true
}

// SetContent replaces the viewport content with the given text.
// Text is word-wrapped at the viewport width before being set.
// Use this for streaming text that should be displayed as continuous prose.
func (o *OutputViewport) SetContent(content string) {
	o.mode = outputModeContent
	o.rawContent = content
	o.rawLines = o.rawLines[:0]

	wrapped := content
	if o.width > 0 {
		// Word-wrap the content at viewport width.
		// lipgloss.NewStyle().Width(w).Render() performs word-wrapping.
		wrapped = lipgloss.NewStyle().Width(o.width).Render(content)
	}

	// Store the wrapped content (now has newlines from wrapping)
	lines := strings.Split(wrapped, "\n")
	if len(lines) > o.maxLines {
		lines = lines[len(lines)-o.maxLines:]
		wrapped = strings.Join(lines, "\n")
	}

	o.lines = lines
	o.viewport.SetContent(wrapped)

	if o.autoScroll {
		o.viewport.GotoBottom()
	}
}

func (o *OutputViewport) rewrap() {
	switch o.mode {
	case outputModeContent:
		o.rewrapContent()
	default:
		o.rewrapLines()
	}
}

func (o *OutputViewport) rewrapLines() {
	if len(o.rawLines) == 0 {
		// Fall back to existing wrapped lines if we don't have raw lines.
		o.viewport.SetContent(strings.Join(o.lines, "\n"))
		return
	}

	var wrappedLines []string
	for _, raw := range o.rawLines {
		wrapped := raw
		if o.width > 0 {
			wrapped = ansi.Wrap(raw, o.width, "/")
		}
		wrappedLines = append(wrappedLines, strings.Split(wrapped, "\n")...)
	}

	if len(wrappedLines) > o.maxLines {
		wrappedLines = wrappedLines[len(wrappedLines)-o.maxLines:]
	}

	o.lines = wrappedLines
	o.viewport.SetContent(strings.Join(o.lines, "\n"))
}

func (o *OutputViewport) rewrapContent() {
	wrapped := o.rawContent
	if o.width > 0 {
		wrapped = lipgloss.NewStyle().Width(o.width).Render(o.rawContent)
	}

	lines := strings.Split(wrapped, "\n")
	if len(lines) > o.maxLines {
		lines = lines[len(lines)-o.maxLines:]
		wrapped = strings.Join(lines, "\n")
	}

	o.lines = lines
	o.viewport.SetContent(wrapped)
}
