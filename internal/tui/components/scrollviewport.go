package components

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const defaultScrollViewportMaxLines = 2000

// ScrollViewport wraps bubbles/viewport.Model with auto-scroll tracking,
// ring buffer for line capping, and scrollbar rendering.
type ScrollViewport struct {
	viewport   viewport.Model
	autoScroll bool     // true = scroll to bottom on new content
	lines      []string // stored lines (ring buffer)
	maxLines   int      // ring buffer capacity
	width      int      // total width including scrollbar
	height     int      // viewport height
}

// NewScrollViewport creates a new ScrollViewport with the given dimensions.
// maxLines controls the ring buffer size (0 uses the default of 2000).
// The width includes 1 column for the scrollbar; the viewport content area
// is width-1.
func NewScrollViewport(width, height, maxLines int) ScrollViewport {
	if maxLines <= 0 {
		maxLines = defaultScrollViewportMaxLines
	}

	contentWidth := width - 1 // 1 col reserved for scrollbar
	if contentWidth < 0 {
		contentWidth = 0
	}

	vp := viewport.New(contentWidth, height)
	vp.SetContent("")

	return ScrollViewport{
		viewport:   vp,
		autoScroll: true,
		lines:      make([]string, 0, 64),
		maxLines:   maxLines,
		width:      width,
		height:     height,
	}
}

// SetSize updates the viewport dimensions. Width includes the scrollbar column.
func (s *ScrollViewport) SetSize(width, height int) {
	if s.width == width && s.height == height {
		return
	}

	s.width = width
	s.height = height

	contentWidth := width - 1
	if contentWidth < 0 {
		contentWidth = 0
	}

	s.viewport.Width = contentWidth
	s.viewport.Height = height

	// Re-set content to let viewport recalculate internal state.
	s.viewport.SetContent(strings.Join(s.lines, "\n"))

	if s.autoScroll {
		s.viewport.GotoBottom()
	} else {
		// Clamp y-offset after resize.
		s.viewport.SetYOffset(s.viewport.YOffset)
	}
}

// SetLines replaces the stored lines, applying ring buffer capping.
// When autoScroll is true, the viewport scrolls to the bottom.
// When autoScroll is false, the viewport preserves the current YOffset.
func (s *ScrollViewport) SetLines(lines []string) {
	if len(lines) > s.maxLines {
		lines = lines[len(lines)-s.maxLines:]
	}

	s.lines = make([]string, len(lines))
	copy(s.lines, lines)

	s.viewport.SetContent(strings.Join(s.lines, "\n"))

	if s.autoScroll {
		s.viewport.GotoBottom()
	} else {
		// Preserve current offset, clamping to valid range.
		s.viewport.SetYOffset(s.viewport.YOffset)
	}
}

// Update handles viewport key and mouse events. Scrolling up pauses
// auto-scroll; returning to the bottom re-enables it.
func (s *ScrollViewport) Update(msg tea.Msg) (ScrollViewport, tea.Cmd) {
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "pgup", "ctrl+u":
			s.autoScroll = false
		case "down", "j", "pgdown", "ctrl+d":
			if s.viewport.AtBottom() {
				s.autoScroll = true
			}
		case "end", "G":
			s.autoScroll = true
		case "home", "g":
			s.autoScroll = false
		}
	case tea.MouseMsg:
		// After mouse scroll, check position to manage auto-scroll.
		if s.viewport.AtBottom() {
			s.autoScroll = true
		} else {
			s.autoScroll = false
		}
	}

	return *s, cmd
}

// View renders the viewport content with a 1-column scrollbar on the right.
func (s ScrollViewport) View() string {
	content := s.viewport.View()
	scrollbar := RenderScrollbar(s.height, len(s.lines), s.viewport.YOffset)

	contentLines := strings.Split(content, "\n")
	scrollbarLines := strings.Split(scrollbar, "\n")

	var b strings.Builder
	for i := 0; i < s.height; i++ {
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
		contentWidth := s.width - 1
		if contentWidth < 0 {
			contentWidth = 0
		}
		padding := contentWidth - utf8.RuneCountInString(cl)
		if padding > 0 {
			b.WriteString(strings.Repeat(" ", padding))
		}
		b.WriteString(sl)
	}

	return b.String()
}

// AtBottom returns true if the viewport is scrolled to the bottom.
func (s ScrollViewport) AtBottom() bool {
	return s.viewport.AtBottom()
}

// ContentWidth returns the width available for content (total width minus scrollbar).
func (s ScrollViewport) ContentWidth() int {
	w := s.width - 1
	if w < 0 {
		return 0
	}
	return w
}

// SetAutoScroll enables or disables auto-scroll. When enabled, the viewport
// scrolls to the bottom immediately.
func (s *ScrollViewport) SetAutoScroll(enabled bool) {
	s.autoScroll = enabled
	if enabled {
		s.viewport.GotoBottom()
	}
}

// AutoScroll returns whether auto-scroll is currently enabled.
func (s ScrollViewport) AutoScroll() bool {
	return s.autoScroll
}

// EnsureVisible scrolls the viewport so that lineIndex is visible. If center
// is true, the line is centered in the viewport; otherwise the viewport
// scrolls the minimum amount needed.
func (s *ScrollViewport) EnsureVisible(lineIndex int, center bool) {
	if lineIndex < 0 || lineIndex >= len(s.lines) {
		return
	}

	top := s.viewport.YOffset
	bottom := top + s.height - 1

	if center {
		target := lineIndex - s.height/2
		if target < 0 {
			target = 0
		}
		s.viewport.SetYOffset(target)
		return
	}

	// Minimum scroll: only move if the line is outside the visible range.
	if lineIndex < top {
		s.viewport.SetYOffset(lineIndex)
	} else if lineIndex > bottom {
		s.viewport.SetYOffset(lineIndex - s.height + 1)
	}
}
