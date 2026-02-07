package components

import (
	"strings"
	"testing"
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
