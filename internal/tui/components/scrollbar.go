package components

import "strings"

// RenderScrollbar renders a 1-column vertical scrollbar. The scrollbar is
// hidden (blank gutter) until content exceeds the viewport height; once
// scrollable, it renders a track and thumb positioned proportionally.
//
// viewHeight is the visible area height, contentHeight is the total number of
// content lines, and yOffset is the current scroll offset (0-based).
//
// Track character: │ (subtle)
// Thumb character: █ (positioned based on scroll percent / visible fraction)
//
// When content fits entirely in the viewport, the scrollbar renders as a blank
// gutter (all spaces) to keep layout width stable without showing a track.
func RenderScrollbar(viewHeight, contentHeight, yOffset int) string {
	if viewHeight <= 0 {
		return ""
	}

	const (
		track = "│"
		thumb = "█"
	)

	// When content fits entirely, render a hidden gutter.
	if contentHeight <= viewHeight {
		return strings.Repeat(" \n", viewHeight-1) + " "
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
