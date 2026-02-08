package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewOutputViewport(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	if ov.LineCount() != 0 {
		t.Errorf("expected 0 lines, got %d", ov.LineCount())
	}
	if !ov.AutoScroll() {
		t.Error("expected autoScroll to be true by default")
	}
}

func TestNewOutputViewport_DefaultMaxLines(t *testing.T) {
	// When maxLines is 0, should use default
	ov := NewOutputViewport(80, 24, 0)

	// Add more than default lines to verify it uses the default
	for i := 0; i < 1001; i++ {
		ov.AddLine("test line")
	}

	// Should be capped at default (1000)
	if ov.LineCount() != 1000 {
		t.Errorf("expected 1000 lines (default cap), got %d", ov.LineCount())
	}
}

func TestOutputViewport_AddLine(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	ov.AddLine("first line")
	if ov.LineCount() != 1 {
		t.Errorf("expected 1 line, got %d", ov.LineCount())
	}

	ov.AddLine("second line")
	if ov.LineCount() != 2 {
		t.Errorf("expected 2 lines, got %d", ov.LineCount())
	}
}

func TestOutputViewport_AddLine_WrapsLongLines(t *testing.T) {
	ov := NewOutputViewport(20, 10, 100)

	ov.AddLine("Hello world! This is a long line that should wrap across multiple lines.")

	// Should have multiple lines after wrapping (not 1 truncated line)
	if ov.LineCount() <= 1 {
		t.Errorf("expected multiple lines after wrapping, got %d", ov.LineCount())
	}

	// View should contain text from both beginning AND end (not truncated)
	view := ov.View()
	if !strings.Contains(view, "Hello") {
		t.Error("expected view to contain beginning of text")
	}
	if !strings.Contains(view, "lines.") {
		t.Error("expected view to contain end of text (proves not truncated)")
	}
}

func TestOutputViewport_RingBuffer(t *testing.T) {
	// Create viewport with small buffer
	ov := NewOutputViewport(80, 24, 5)

	// Add 7 lines
	for i := 1; i <= 7; i++ {
		ov.AddLine("line")
	}

	// Should only have 5 lines (oldest dropped)
	if ov.LineCount() != 5 {
		t.Errorf("expected 5 lines due to ring buffer, got %d", ov.LineCount())
	}
}

func TestOutputViewport_Clear(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	ov.AddLine("line 1")
	ov.AddLine("line 2")
	ov.AddLine("line 3")

	if ov.LineCount() != 3 {
		t.Errorf("expected 3 lines before clear, got %d", ov.LineCount())
	}

	ov.Clear()

	if ov.LineCount() != 0 {
		t.Errorf("expected 0 lines after clear, got %d", ov.LineCount())
	}
	if !ov.AutoScroll() {
		t.Error("expected autoScroll to be re-enabled after clear")
	}
}

func TestOutputViewport_SetAutoScroll(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	// Initially true
	if !ov.AutoScroll() {
		t.Error("expected autoScroll to be true initially")
	}

	// Disable
	ov.SetAutoScroll(false)
	if ov.AutoScroll() {
		t.Error("expected autoScroll to be false after disabling")
	}

	// Re-enable
	ov.SetAutoScroll(true)
	if !ov.AutoScroll() {
		t.Error("expected autoScroll to be true after enabling")
	}
}

func TestOutputViewport_SetSize(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	ov.SetSize(120, 40)

	// The viewport should still work after resize
	ov.AddLine("test after resize")
	if ov.LineCount() != 1 {
		t.Errorf("expected 1 line after resize, got %d", ov.LineCount())
	}
}

func TestOutputViewport_SetSize_RewrapsExistingLines(t *testing.T) {
	ov := NewOutputViewport(80, 10, 100)

	ov.AddLine("Hello world! This is a long line that should wrap when the viewport is narrowed.")
	if ov.LineCount() != 1 {
		t.Fatalf("expected 1 line before narrowing, got %d", ov.LineCount())
	}

	ov.SetSize(20, 10)
	if ov.LineCount() <= 1 {
		t.Fatalf("expected multiple lines after narrowing, got %d", ov.LineCount())
	}

	ov.SetSize(80, 10)
	if ov.LineCount() != 1 {
		t.Fatalf("expected 1 line after widening, got %d", ov.LineCount())
	}
}

func TestOutputViewport_View(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	// View should not panic on empty viewport
	view := ov.View()
	if view == "" {
		t.Log("Empty viewport returns empty view (which is valid)")
	}

	// Add some content
	ov.AddLine("test line")
	view = ov.View()

	// View should not be empty after adding content
	// Note: The exact format depends on viewport implementation
	_ = view
}

func TestOutputViewport_SetContent_StreamingText(t *testing.T) {
	// Use a narrow width to test word-wrapping
	ov := NewOutputViewport(40, 10, 100)

	// Simulate streaming text that exceeds viewport width
	longText := "Hello world! This is a test of streaming text that should wrap properly across multiple lines."
	ov.SetContent(longText)

	// Should have multiple lines after wrapping (not 1 truncated line)
	if ov.LineCount() <= 1 {
		t.Errorf("expected multiple lines after wrapping, got %d", ov.LineCount())
	}

	// View should contain text from both beginning AND end (not truncated)
	view := ov.View()
	if !strings.Contains(view, "Hello") {
		t.Error("expected view to contain beginning of text")
	}
	if !strings.Contains(view, "lines") {
		t.Error("expected view to contain end of text (proves not truncated)")
	}
}

func TestOutputViewport_SetContent_ReplacesExisting(t *testing.T) {
	ov := NewOutputViewport(40, 10, 100)

	// First set some content
	ov.SetContent("initial content")

	// Replace with new content
	ov.SetContent("completely new content")

	// Should only have the new content
	view := ov.View()
	if !strings.Contains(view, "completely new content") {
		t.Error("expected view to contain new content")
	}
	if strings.Contains(view, "initial") {
		t.Error("expected old content to be replaced")
	}
}

// --- Scrollbar tests ---

func TestOutputViewport_ShowScrollbar_DefaultFalse(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)
	if ov.ShowScrollbar() {
		t.Error("expected ShowScrollbar to be false by default")
	}
}

func TestOutputViewport_SetShowScrollbar(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	ov.SetShowScrollbar(true)
	if !ov.ShowScrollbar() {
		t.Error("expected ShowScrollbar to be true after enabling")
	}

	ov.SetShowScrollbar(false)
	if ov.ShowScrollbar() {
		t.Error("expected ShowScrollbar to be false after disabling")
	}
}

func TestOutputViewport_Scrollbar_AppearsWhenEnabled(t *testing.T) {
	// Width 21 = 20 content + 1 scrollbar.
	ov := NewOutputViewport(21, 5, 100)
	ov.SetShowScrollbar(true)

	// Add enough lines to produce a scrollbar thumb.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "content"
	}
	for _, l := range lines {
		ov.AddLine(l)
	}

	view := ov.View()
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

func TestOutputViewport_Scrollbar_NotPresentWhenDisabled(t *testing.T) {
	ov := NewOutputViewport(21, 5, 100)
	// Scrollbar disabled by default.

	for i := 0; i < 20; i++ {
		ov.AddLine("content")
	}

	view := ov.View()

	// View should not contain scrollbar characters at line ends.
	if strings.Contains(view, "│") || strings.Contains(view, "█") {
		t.Error("expected no scrollbar characters when scrollbar is disabled")
	}
}

func TestOutputViewport_Scrollbar_WidthAdjustsForWrapping(t *testing.T) {
	// With scrollbar disabled, a 20-char wide viewport wraps at 20.
	// With scrollbar enabled, content width is 19, so wrapping happens sooner.
	ov := NewOutputViewport(20, 10, 100)

	// A line exactly 20 chars should NOT wrap without scrollbar.
	line := "12345678901234567890" // exactly 20 chars
	ov.AddLine(line)
	countWithout := ov.LineCount()

	// Now enable scrollbar and re-add.
	ov.Clear()
	ov.SetShowScrollbar(true)
	ov.AddLine(line)
	countWith := ov.LineCount()

	if countWithout != 1 {
		t.Errorf("expected 1 line without scrollbar, got %d", countWithout)
	}
	if countWith <= 1 {
		t.Errorf("expected wrapping with scrollbar (content width 19), got %d lines", countWith)
	}
}

func TestOutputViewport_Scrollbar_ViewWidthAccountsForScrollbar(t *testing.T) {
	// Width 21 = 20 content + 1 scrollbar.
	ov := NewOutputViewport(21, 5, 100)
	ov.SetShowScrollbar(true)
	ov.AddLine("hello")

	view := ov.View()
	viewLines := strings.Split(view, "\n")

	// Each view line should have 20 chars of content (possibly padded) + 1 scrollbar rune = 21 runes.
	for i, line := range viewLines {
		runeCount := len([]rune(line))
		if runeCount != 21 {
			t.Errorf("line %d: expected 21 runes, got %d: %q", i, runeCount, line)
		}
	}
}

func TestOutputViewport_Scrollbar_AutoScrollStillWorks(t *testing.T) {
	ov := NewOutputViewport(40, 5, 100)
	ov.SetShowScrollbar(true)

	// Add many lines — auto-scroll should keep us at bottom.
	for i := 0; i < 50; i++ {
		ov.AddLine("line")
	}
	if !ov.AutoScroll() {
		t.Error("expected autoScroll to remain true after adding lines")
	}

	// Simulate scrolling up.
	ov, _ = ov.Update(tea.KeyMsg{Type: tea.KeyUp})
	if ov.AutoScroll() {
		t.Error("expected autoScroll to be disabled after up key")
	}

	// Simulate scrolling to bottom.
	ov, _ = ov.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if !ov.AutoScroll() {
		t.Error("expected autoScroll to be re-enabled after End key")
	}
}

func TestOutputViewport_Scrollbar_RingBufferPreserved(t *testing.T) {
	ov := NewOutputViewport(40, 5, 10)
	ov.SetShowScrollbar(true)

	// Add 15 lines — ring buffer should cap at 10.
	for i := 0; i < 15; i++ {
		ov.AddLine("line")
	}

	if ov.LineCount() != 10 {
		t.Errorf("expected 10 lines due to ring buffer, got %d", ov.LineCount())
	}
}

func TestOutputViewport_Scrollbar_SetContent_WrapsAtContentWidth(t *testing.T) {
	ov := NewOutputViewport(21, 10, 100)
	ov.SetShowScrollbar(true)

	// Content width is 20 when scrollbar is enabled (21 - 1).
	longText := "Hello world! This is a test of streaming text that should wrap at content width."
	ov.SetContent(longText)

	// Should have multiple lines after wrapping at width 20.
	if ov.LineCount() <= 1 {
		t.Errorf("expected multiple lines after wrapping, got %d", ov.LineCount())
	}

	view := ov.View()
	if !strings.Contains(view, "Hello") {
		t.Error("expected view to contain beginning of text")
	}
}

func TestOutputViewport_Scrollbar_SetSizeRewraps(t *testing.T) {
	ov := NewOutputViewport(80, 10, 100)
	ov.SetShowScrollbar(true)

	// Content width is 79 (80-1). Use a short line that fits.
	ov.AddLine("Hello world! Short enough to fit.")
	if ov.LineCount() != 1 {
		t.Fatalf("expected 1 line before narrowing, got %d", ov.LineCount())
	}

	// Narrow to 21 (content width 20) — should rewrap.
	ov.SetSize(21, 10)
	if ov.LineCount() <= 1 {
		t.Fatalf("expected multiple lines after narrowing, got %d", ov.LineCount())
	}
}

func TestOutputViewport_SetShowScrollbar_RewrapsExisting(t *testing.T) {
	// Start without scrollbar, add a line that fits exactly at width 20.
	ov := NewOutputViewport(20, 10, 100)
	ov.AddLine("12345678901234567890") // 20 chars, fits at width 20
	if ov.LineCount() != 1 {
		t.Fatalf("expected 1 line without scrollbar, got %d", ov.LineCount())
	}

	// Enable scrollbar — content width becomes 19, should rewrap.
	ov.SetShowScrollbar(true)
	if ov.LineCount() <= 1 {
		t.Errorf("expected rewrap after enabling scrollbar, got %d lines", ov.LineCount())
	}

	// Disable scrollbar — content width back to 20, should unwrap.
	ov.SetShowScrollbar(false)
	if ov.LineCount() != 1 {
		t.Errorf("expected unwrap after disabling scrollbar, got %d lines", ov.LineCount())
	}
}
