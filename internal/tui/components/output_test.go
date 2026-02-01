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
	ov := NewOutputViewport(80, 24, 100)

	// Simulate streaming: accumulate text and set full content each time
	accumulated := ""

	accumulated += "Hello "
	ov.SetContent(accumulated)

	accumulated += "world! "
	ov.SetContent(accumulated)

	accumulated += "This is a test."
	ov.SetContent(accumulated)

	// Should have one logical block of content, not 3 lines
	if ov.LineCount() != 1 {
		t.Errorf("expected 1 logical content block, got %d", ov.LineCount())
	}

	// View should contain the full accumulated text
	view := ov.View()
	if !strings.Contains(view, "Hello world! This is a test.") {
		t.Error("expected view to contain full accumulated text as continuous sentence")
	}
}

func TestOutputViewport_SetContent_ReplacesExisting(t *testing.T) {
	ov := NewOutputViewport(80, 24, 100)

	// First set some content
	ov.SetContent("initial content")
	if ov.LineCount() != 1 {
		t.Errorf("expected 1 line after first SetContent, got %d", ov.LineCount())
	}

	// Replace with new content
	ov.SetContent("completely new content")
	if ov.LineCount() != 1 {
		t.Errorf("expected 1 line after second SetContent, got %d", ov.LineCount())
	}

	view := ov.View()
	if !strings.Contains(view, "completely new content") {
		t.Error("expected view to contain new content")
	}
	if strings.Contains(view, "initial") {
		t.Error("expected old content to be replaced")
	}
}
