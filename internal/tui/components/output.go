package components

import (
	"strings"
	"unicode/utf8"

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
	viewport      viewport.Model
	autoScroll    bool     // true = scroll to bottom on new content
	rawLines      []string // buffered raw lines (unwrapped)
	rawLineOpen   bool     // true when the last raw line is an unfinished chunk line
	rawContent    string   // raw content when using SetContent
	lines         []string // buffered lines
	maxLines      int      // ring buffer size
	width         int
	height        int
	mode          outputMode
	showScrollbar bool // when true, reserve 1 column for a scrollbar
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

// SetShowScrollbar enables or disables the scrollbar. When enabled, 1 column
// is reserved for the scrollbar and the content/wrapping width is reduced
// accordingly. Existing content is rewrapped after the change.
func (o *OutputViewport) SetShowScrollbar(enabled bool) {
	if o.showScrollbar == enabled {
		return
	}
	o.showScrollbar = enabled
	// Update viewport width and rewrap.
	cw := o.contentWidth()
	o.viewport.Width = cw
	o.rewrap()
	if o.autoScroll {
		o.viewport.GotoBottom()
	}
}

// ShowScrollbar returns whether the scrollbar is enabled.
func (o OutputViewport) ShowScrollbar() bool {
	return o.showScrollbar
}

// contentWidth returns the width available for content (minus scrollbar if enabled).
func (o OutputViewport) contentWidth() int {
	if o.showScrollbar {
		w := o.width - 1
		if w < 0 {
			return 0
		}
		return w
	}
	return o.width
}

// AddLine appends a complete line to the buffer.
func (o *OutputViewport) AddLine(line string) {
	o.mode = outputModeLines
	o.rawContent = ""
	o.rawLineOpen = false
	o.appendRawLine(line)
	o.rewrapLines()

	// Auto-scroll to bottom if enabled
	if o.autoScroll {
		o.viewport.GotoBottom()
	}
}

// AppendChunk appends streamed output chunks while preserving exact whitespace
// and newlines. Chunk boundaries do not create hard line breaks.
func (o *OutputViewport) AppendChunk(chunk string) {
	if chunk == "" {
		return
	}

	o.mode = outputModeLines
	o.rawContent = ""

	start := 0
	for start < len(chunk) {
		relIdx := strings.IndexByte(chunk[start:], '\n')
		if relIdx == -1 {
			o.appendToCurrentRawLine(chunk[start:])
			break
		}

		segment := chunk[start : start+relIdx]
		o.appendToCurrentRawLine(segment)
		o.rawLineOpen = false
		start += relIdx + 1
	}

	o.rewrapLines()

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

// ViewContent returns the raw viewport content without the scrollbar column.
// Use this when you need to post-process the content before rendering
// the final output with ComposeWithScrollbar.
func (o OutputViewport) ViewContent() string {
	return o.viewport.View()
}

// ComposeWithScrollbar combines the given content string with a scrollbar column.
// This is useful when the caller needs to modify the viewport content (e.g., insert
// a spinner) before the scrollbar is appended. If the scrollbar is disabled, the
// content is returned unchanged.
func (o OutputViewport) ComposeWithScrollbar(content string) string {
	if !o.showScrollbar {
		return content
	}

	scrollbar := RenderScrollbar(o.height, len(o.lines), o.viewport.YOffset)

	contentLines := strings.Split(content, "\n")
	scrollbarLines := strings.Split(scrollbar, "\n")

	cw := o.contentWidth()

	var b strings.Builder
	for i := 0; i < o.height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}

		cl := ""
		if i < len(contentLines) {
			cl = contentLines[i]
		}
		sl := ""
		if i < len(scrollbarLines) {
			sl = scrollbarLines[i]
		}

		b.WriteString(cl)
		// Pad content to fill the content width so the scrollbar aligns.
		padding := cw - utf8.RuneCountInString(cl)
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteString(sl)
	}

	return b.String()
}

// View returns the rendered viewport, optionally with a scrollbar column.
func (o OutputViewport) View() string {
	return o.ComposeWithScrollbar(o.viewport.View())
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
	o.viewport.Width = o.contentWidth()
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
	o.rawLineOpen = false
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
	o.rawLineOpen = false

	cw := o.contentWidth()
	wrapped := content
	if cw > 0 {
		// Word-wrap the content at content width.
		// lipgloss.NewStyle().Width(w).Render() performs word-wrapping.
		wrapped = lipgloss.NewStyle().Width(cw).Render(content)
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

	cw := o.contentWidth()
	var wrappedLines []string
	for _, raw := range o.rawLines {
		wrapped := raw
		if cw > 0 {
			wrapped = ansi.Wrap(raw, cw, "/")
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
	cw := o.contentWidth()
	wrapped := o.rawContent
	if cw > 0 {
		wrapped = lipgloss.NewStyle().Width(cw).Render(o.rawContent)
	}

	lines := strings.Split(wrapped, "\n")
	if len(lines) > o.maxLines {
		lines = lines[len(lines)-o.maxLines:]
		wrapped = strings.Join(lines, "\n")
	}

	o.lines = lines
	o.viewport.SetContent(wrapped)
}

func (o *OutputViewport) appendRawLine(line string) {
	if len(o.rawLines) >= o.maxLines {
		o.rawLines = o.rawLines[1:]
	}
	o.rawLines = append(o.rawLines, line)
}

func (o *OutputViewport) appendToCurrentRawLine(text string) {
	if o.rawLineOpen && len(o.rawLines) > 0 {
		o.rawLines[len(o.rawLines)-1] += text
		return
	}

	o.appendRawLine(text)
	o.rawLineOpen = true
}
