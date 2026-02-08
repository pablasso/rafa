package components

import "strings"

// RenderScrollbar renders a 1-column vertical scrollbar with track and thumb
// characters positioned proportionally. viewHeight is the visible area height,
// contentHeight is the total number of content lines, and yOffset is the
// current scroll offset (0-based).
//
// Track character: │ (subtle)
// Thumb character: █ (positioned based on scroll percent / visible fraction)
//
// When content fits entirely in the viewport, the scrollbar renders as an
// empty track (all │ characters).
func RenderScrollbar(viewHeight, contentHeight, yOffset int) string {
	if viewHeight <= 0 {
		return ""
	}

	const (
		track = "│"
		thumb = "█"
	)

	// When content fits entirely, render empty track.
	if contentHeight <= viewHeight {
		return strings.Repeat(track+"\n", viewHeight-1) + track
	}

	// Thumb size proportional to visible fraction, minimum 1.
	thumbSize := viewHeight * viewHeight / contentHeight
	if thumbSize < 1 {
		thumbSize = 1
	}

	// Thumb position.
	maxYOffset := contentHeight - viewHeight
	thumbMaxTop := viewHeight - thumbSize

	thumbTop := 0
	if maxYOffset > 0 {
		thumbTop = yOffset * thumbMaxTop / maxYOffset
	}
	// Clamp to valid range.
	if thumbTop > thumbMaxTop {
		thumbTop = thumbMaxTop
	}
	if thumbTop < 0 {
		thumbTop = 0
	}

	var b strings.Builder
	for i := 0; i < viewHeight; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i >= thumbTop && i < thumbTop+thumbSize {
			b.WriteString(thumb)
		} else {
			b.WriteString(track)
		}
	}

	return b.String()
}
