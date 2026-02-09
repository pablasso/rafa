package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- RenderScrollbar tests ---

func TestRenderScrollbar_ZeroHeight(t *testing.T) {
	result := RenderScrollbar(0, 100, 0)
	if result != "" {
		t.Errorf("expected empty string for zero height, got %q", result)
	}
}

func TestRenderScrollbar_ContentFitsInViewport(t *testing.T) {
	result := RenderScrollbar(10, 5, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// All lines should be blank gutter spaces when content fits.
	for i, line := range lines {
		if line != " " {
			t.Errorf("line %d: expected blank gutter space, got %q", i, line)
		}
	}
}

func TestRenderScrollbar_ContentEqualsViewport(t *testing.T) {
	result := RenderScrollbar(10, 10, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Content equals viewport: should be hidden gutter (no scrolling needed).
	for i, line := range lines {
		if line != " " {
			t.Errorf("line %d: expected blank gutter space, got %q", i, line)
		}
	}
}

func TestRenderScrollbar_ThumbAtTop(t *testing.T) {
	// 10 view lines, 100 content lines, offset 0 (at top).
	result := RenderScrollbar(10, 100, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Thumb should be at line 0 (first line).
	if lines[0] != "█" {
		t.Errorf("expected thumb █ at line 0, got %q", lines[0])
	}
	// Lines after thumb should be track.
	for i := 1; i < 10; i++ {
		if lines[i] != "│" {
			t.Errorf("line %d: expected track │, got %q", i, lines[i])
		}
	}
}

func TestRenderScrollbar_ThumbAtBottom(t *testing.T) {
	// 10 view lines, 100 content lines, offset at max (90).
	result := RenderScrollbar(10, 100, 90)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Thumb should be at the last line.
	if lines[9] != "█" {
		t.Errorf("expected thumb █ at line 9, got %q", lines[9])
	}
	// Lines before thumb should be track.
	for i := 0; i < 9; i++ {
		if lines[i] != "│" {
			t.Errorf("line %d: expected track │, got %q", i, lines[i])
		}
	}
}

func TestRenderScrollbar_ThumbAtMiddle(t *testing.T) {
	// 10 view lines, 100 content lines, offset 45 (middle).
	result := RenderScrollbar(10, 100, 45)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}

	// Find the thumb position.
	thumbPos := -1
	for i, line := range lines {
		if line == "█" {
			thumbPos = i
			break
		}
	}
	if thumbPos == -1 {
		t.Fatal("expected to find thumb character █")
	}
	// Thumb should be in the middle region (4 or 5).
	if thumbPos < 3 || thumbPos > 6 {
		t.Errorf("expected thumb near middle (3-6), got position %d", thumbPos)
	}
}

func TestRenderScrollbar_LargerThumb(t *testing.T) {
	// 10 view lines, 20 content lines → thumb size = 10*10/20 = 5.
	result := RenderScrollbar(10, 20, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}

	thumbCount := 0
	for _, line := range lines {
		if line == "█" {
			thumbCount++
		}
	}
	if thumbCount != 5 {
		t.Errorf("expected thumb size 5, got %d", thumbCount)
	}
}

// --- ScrollViewport tests ---

func TestNewScrollViewport(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)

	if !sv.AutoScroll() {
		t.Error("expected autoScroll to be true by default")
	}
	if !sv.AtBottom() {
		t.Error("expected to be at bottom with no content")
	}
}

func TestNewScrollViewport_DefaultMaxLines(t *testing.T) {
	sv := NewScrollViewport(80, 24, 0)

	lines := make([]string, 2001)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	// Should be capped at default (2000).
	if len(sv.lines) != 2000 {
		t.Errorf("expected 2000 lines (default cap), got %d", len(sv.lines))
	}
}

func TestScrollViewport_SetLines(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)

	sv.SetLines([]string{"line 1", "line 2", "line 3"})

	if len(sv.lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(sv.lines))
	}
}

func TestScrollViewport_SetLines_RingBuffer(t *testing.T) {
	sv := NewScrollViewport(80, 24, 5)

	lines := []string{"a", "b", "c", "d", "e", "f", "g"}
	sv.SetLines(lines)

	// Should keep only last 5.
	if len(sv.lines) != 5 {
		t.Errorf("expected 5 lines due to ring buffer, got %d", len(sv.lines))
	}
	if sv.lines[0] != "c" {
		t.Errorf("expected first line to be 'c', got %q", sv.lines[0])
	}
	if sv.lines[4] != "g" {
		t.Errorf("expected last line to be 'g', got %q", sv.lines[4])
	}
}

func TestScrollViewport_SetLines_PreservesYOffset_WhenAutoScrollDisabled(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	// Set initial content longer than viewport.
	initial := make([]string, 20)
	for i := range initial {
		initial[i] = "initial line"
	}
	sv.SetLines(initial)

	// Disable auto-scroll and scroll to a specific position.
	sv.SetAutoScroll(false)
	sv.viewport.SetYOffset(5)

	// Set new lines — offset should be preserved.
	updated := make([]string, 25)
	for i := range updated {
		updated[i] = "updated line"
	}
	sv.SetLines(updated)

	if sv.viewport.YOffset != 5 {
		t.Errorf("expected YOffset=5 to be preserved, got %d", sv.viewport.YOffset)
	}
}

func TestScrollViewport_AutoScroll_DisablesOnUpKey(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	// Add enough content to scroll.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	if !sv.AutoScroll() {
		t.Fatal("expected autoScroll to be true initially")
	}

	// Simulate pressing up.
	sv, _ = sv.Update(tea.KeyMsg{Type: tea.KeyUp})

	if sv.AutoScroll() {
		t.Error("expected autoScroll to be disabled after up key")
	}
}

func TestScrollViewport_AutoScroll_ReenablesOnEndKey(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	// Disable auto-scroll.
	sv, _ = sv.Update(tea.KeyMsg{Type: tea.KeyUp})
	if sv.AutoScroll() {
		t.Fatal("expected autoScroll to be disabled after up")
	}

	// Press End to go to bottom.
	sv, _ = sv.Update(tea.KeyMsg{Type: tea.KeyEnd})

	if !sv.AutoScroll() {
		t.Error("expected autoScroll to be re-enabled after End key")
	}
}

func TestScrollViewport_AutoScroll_ReenablesOnGKey(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	// Disable auto-scroll.
	sv.SetAutoScroll(false)

	// Press G to go to bottom.
	sv, _ = sv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	if !sv.AutoScroll() {
		t.Error("expected autoScroll to be re-enabled after G key")
	}
}

func TestScrollViewport_AutoScroll_HomeKeyDisables(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	// Press Home.
	sv, _ = sv.Update(tea.KeyMsg{Type: tea.KeyHome})

	if sv.AutoScroll() {
		t.Error("expected autoScroll to be disabled after Home key")
	}
}

func TestScrollViewport_SetAutoScroll(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)

	if !sv.AutoScroll() {
		t.Error("expected autoScroll true initially")
	}

	sv.SetAutoScroll(false)
	if sv.AutoScroll() {
		t.Error("expected autoScroll false after disabling")
	}

	sv.SetAutoScroll(true)
	if !sv.AutoScroll() {
		t.Error("expected autoScroll true after enabling")
	}
}

func TestScrollViewport_SetSize(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)

	sv.SetSize(120, 40)

	sv.SetLines([]string{"test after resize"})
	if len(sv.lines) != 1 {
		t.Errorf("expected 1 line after resize, got %d", len(sv.lines))
	}
}

func TestScrollViewport_SetSize_NoOp(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)
	sv.SetLines([]string{"line 1", "line 2"})

	// Same size → should not change anything.
	sv.SetSize(80, 24)
	if len(sv.lines) != 2 {
		t.Errorf("expected 2 lines unchanged, got %d", len(sv.lines))
	}
}

func TestScrollViewport_EnsureVisible_BelowViewport(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	// Start at top.
	sv.SetAutoScroll(false)
	sv.viewport.SetYOffset(0)

	// Ensure line 15 is visible (currently below viewport).
	sv.EnsureVisible(15, false)

	top := sv.viewport.YOffset
	bottom := top + sv.height - 1
	if 15 < top || 15 > bottom {
		t.Errorf("line 15 should be visible, viewport range [%d, %d]", top, bottom)
	}
}

func TestScrollViewport_EnsureVisible_AboveViewport(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	// Scroll to bottom first.
	sv.SetAutoScroll(false)
	sv.viewport.SetYOffset(15)

	// Ensure line 2 is visible (currently above viewport).
	sv.EnsureVisible(2, false)

	if sv.viewport.YOffset != 2 {
		t.Errorf("expected YOffset=2 to make line 2 visible at top, got %d", sv.viewport.YOffset)
	}
}

func TestScrollViewport_EnsureVisible_AlreadyVisible(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	sv.SetAutoScroll(false)
	sv.viewport.SetYOffset(5)

	// Line 7 should already be visible (viewport shows lines 5-9).
	sv.EnsureVisible(7, false)

	if sv.viewport.YOffset != 5 {
		t.Errorf("expected YOffset=5 unchanged, got %d", sv.viewport.YOffset)
	}
}

func TestScrollViewport_EnsureVisible_Center(t *testing.T) {
	sv := NewScrollViewport(80, 10, 100)

	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	sv.SetAutoScroll(false)
	sv.viewport.SetYOffset(0)

	// Center line 15 in a height-10 viewport.
	sv.EnsureVisible(15, true)

	// Expected offset: 15 - 10/2 = 10.
	if sv.viewport.YOffset != 10 {
		t.Errorf("expected YOffset=10 to center line 15, got %d", sv.viewport.YOffset)
	}
}

func TestScrollViewport_EnsureVisible_CenterClampToZero(t *testing.T) {
	sv := NewScrollViewport(80, 10, 100)

	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	sv.SetAutoScroll(false)
	sv.viewport.SetYOffset(15)

	// Center line 2 → target = 2 - 5 = -3, clamped to 0.
	sv.EnsureVisible(2, true)

	if sv.viewport.YOffset != 0 {
		t.Errorf("expected YOffset=0 (clamped), got %d", sv.viewport.YOffset)
	}
}

func TestScrollViewport_EnsureVisible_InvalidIndex(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)
	sv.SetLines([]string{"a", "b", "c"})

	sv.viewport.SetYOffset(0)

	// Negative index: should be a no-op.
	sv.EnsureVisible(-1, false)
	if sv.viewport.YOffset != 0 {
		t.Errorf("expected no change for negative index, got YOffset %d", sv.viewport.YOffset)
	}

	// Out of range: should be a no-op.
	sv.EnsureVisible(10, false)
	if sv.viewport.YOffset != 0 {
		t.Errorf("expected no change for out-of-range index, got YOffset %d", sv.viewport.YOffset)
	}
}

func TestScrollViewport_View_ContainsScrollbar(t *testing.T) {
	sv := NewScrollViewport(20, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "content"
	}
	sv.SetLines(lines)

	view := sv.View()
	viewLines := strings.Split(view, "\n")

	if len(viewLines) != 5 {
		t.Fatalf("expected 5 view lines, got %d", len(viewLines))
	}

	// Each line should end with a scrollbar character (█ or │).
	for i, line := range viewLines {
		if !strings.HasSuffix(line, "█") && !strings.HasSuffix(line, "│") {
			t.Errorf("line %d should end with scrollbar char, got %q", i, line)
		}
	}
}

func TestScrollViewport_View_ShortContent(t *testing.T) {
	sv := NewScrollViewport(20, 10, 100)

	// Only 3 lines, viewport is 10 lines tall.
	sv.SetLines([]string{"a", "b", "c"})

	view := sv.View()
	viewLines := strings.Split(view, "\n")

	if len(viewLines) != 10 {
		t.Fatalf("expected 10 view lines, got %d", len(viewLines))
	}

	// Scrollbar gutter should be hidden when content fits.
	for i, line := range viewLines {
		if !strings.HasSuffix(line, " ") {
			t.Errorf("line %d: expected hidden scrollbar gutter for short content, got %q", i, line)
		}
	}
}

func TestScrollViewport_View_WidthAccountsForScrollbar(t *testing.T) {
	// Width 21 = 20 content + 1 scrollbar.
	sv := NewScrollViewport(21, 5, 100)
	sv.SetLines([]string{"hello"})

	view := sv.View()
	viewLines := strings.Split(view, "\n")

	// Each view line should have 20 chars of content (possibly padded) + 1 scrollbar rune.
	// The scrollbar rune (│) is 3 bytes in UTF-8, so byte length = 20 + 3 = 23.
	for i, line := range viewLines {
		runeCount := len([]rune(line))
		if runeCount != 21 {
			t.Errorf("line %d: expected 21 runes, got %d: %q", i, runeCount, line)
		}
	}
}

// --- Edge Case Tests ---

func TestNewScrollViewport_ZeroWidth(t *testing.T) {
	// Zero width should not panic
	sv := NewScrollViewport(0, 10, 100)
	if sv.ContentWidth() != 0 {
		t.Errorf("expected content width 0 for zero width, got %d", sv.ContentWidth())
	}

	sv.SetLines([]string{"a", "b"})
	view := sv.View()
	_ = view // just verify no panic
}

func TestNewScrollViewport_NegativeWidth(t *testing.T) {
	// Negative width should clamp to 0 content width
	sv := NewScrollViewport(-1, 10, 100)
	if sv.ContentWidth() != 0 {
		t.Errorf("expected content width 0 for negative width, got %d", sv.ContentWidth())
	}
}

func TestNewScrollViewport_ZeroHeight(t *testing.T) {
	sv := NewScrollViewport(80, 0, 100)
	sv.SetLines([]string{"a", "b"})
	view := sv.View()
	if view != "" {
		t.Errorf("expected empty view for zero height, got %q", view)
	}
}

func TestNewScrollViewport_OneByOne(t *testing.T) {
	// Minimal dimensions: width 1 means content width 0 (scrollbar takes 1 col)
	sv := NewScrollViewport(1, 1, 100)
	sv.SetLines([]string{"a"})
	view := sv.View()
	// Should render without panic
	_ = view
}

func TestScrollViewport_SetSize_VerySmall(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)
	sv.SetLines([]string{"a", "b", "c"})

	// Resize to very small — should not panic
	sv.SetSize(1, 1)
	view := sv.View()
	_ = view
}

func TestScrollViewport_SetSize_ZeroWidth(t *testing.T) {
	sv := NewScrollViewport(80, 24, 100)
	sv.SetLines([]string{"a", "b", "c"})

	sv.SetSize(0, 10)
	if sv.ContentWidth() != 0 {
		t.Errorf("expected content width 0, got %d", sv.ContentWidth())
	}
}

func TestScrollViewport_SetLines_Empty(t *testing.T) {
	sv := NewScrollViewport(80, 10, 100)
	sv.SetLines([]string{})

	if len(sv.lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(sv.lines))
	}

	view := sv.View()
	_ = view // should not panic
}

func TestScrollViewport_ContentShorterThanViewport_ScrollbarHidden(t *testing.T) {
	sv := NewScrollViewport(20, 10, 100)

	// Only 3 lines in a 10-line viewport
	sv.SetLines([]string{"one", "two", "three"})

	view := sv.View()
	viewLines := strings.Split(view, "\n")

	if len(viewLines) != 10 {
		t.Fatalf("expected 10 view lines, got %d", len(viewLines))
	}

	// Hidden gutter should render spaces, not visible scrollbar characters.
	for i, line := range viewLines {
		if strings.HasSuffix(line, "█") || strings.HasSuffix(line, "│") {
			t.Errorf("line %d: expected hidden scrollbar gutter for short content, got %q", i, line)
		}
		if !strings.HasSuffix(line, " ") {
			t.Errorf("line %d: expected trailing hidden-gutter space, got %q", i, line)
		}
	}
}

func TestScrollViewport_SetLines_PreservesAutoScroll_WhenNewContentAdded(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	// Set initial content and verify at bottom
	initial := make([]string, 10)
	for i := range initial {
		initial[i] = "line"
	}
	sv.SetLines(initial)

	if !sv.AutoScroll() {
		t.Error("expected autoScroll to be true initially")
	}

	// Add more content — should still auto-scroll to bottom
	updated := make([]string, 20)
	for i := range updated {
		updated[i] = "updated"
	}
	sv.SetLines(updated)

	if !sv.AutoScroll() {
		t.Error("expected autoScroll to remain true when content grows")
	}
	if !sv.AtBottom() {
		t.Error("expected viewport to be at bottom after autoScroll content update")
	}
}

func TestScrollViewport_MouseWheel_AutoScrollBehavior(t *testing.T) {
	sv := NewScrollViewport(80, 5, 100)

	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	sv.SetLines(lines)

	if !sv.AutoScroll() {
		t.Fatal("expected autoScroll true initially")
	}

	// Simulate mouse wheel up
	sv, _ = sv.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})

	// After scrolling up, should not be at bottom anymore → autoScroll false
	if sv.AutoScroll() {
		t.Error("expected autoScroll to be disabled after mouse wheel up")
	}
}

func TestRenderScrollbar_NegativeHeight(t *testing.T) {
	result := RenderScrollbar(-1, 100, 0)
	if result != "" {
		t.Errorf("expected empty string for negative height, got %q", result)
	}
}

func TestRenderScrollbar_ZeroContent(t *testing.T) {
	result := RenderScrollbar(10, 0, 0)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// All should be hidden gutter spaces.
	for i, line := range lines {
		if line != " " {
			t.Errorf("line %d: expected blank gutter space, got %q", i, line)
		}
	}
}

func TestRenderScrollbar_NegativeOffset(t *testing.T) {
	// Negative offset should be clamped to 0
	result := RenderScrollbar(10, 100, -5)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Thumb should be at top
	if lines[0] != "█" {
		t.Errorf("expected thumb at line 0 for negative offset, got %q", lines[0])
	}
}
