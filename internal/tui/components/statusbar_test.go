package components

import (
	"strings"
	"testing"
)

func TestStatusBar_Render_SingleItem(t *testing.T) {
	sb := NewStatusBar()
	result := sb.Render(50, []string{"q Quit"})

	if !strings.Contains(result, "q Quit") {
		t.Errorf("expected result to contain 'q Quit', got: %s", result)
	}
}

func TestStatusBar_Render_MultipleItems(t *testing.T) {
	sb := NewStatusBar()
	items := []string{"↑↓ Navigate", "Enter Select", "q Quit"}
	result := sb.Render(60, items)

	// Should contain all items with separator
	if !strings.Contains(result, "↑↓ Navigate") {
		t.Errorf("expected result to contain '↑↓ Navigate', got: %s", result)
	}
	if !strings.Contains(result, "Enter Select") {
		t.Errorf("expected result to contain 'Enter Select', got: %s", result)
	}
	if !strings.Contains(result, "q Quit") {
		t.Errorf("expected result to contain 'q Quit', got: %s", result)
	}
	if !strings.Contains(result, "|") {
		t.Errorf("expected result to contain '|' separator, got: %s", result)
	}
}

func TestStatusBar_Render_EmptyItems(t *testing.T) {
	sb := NewStatusBar()
	result := sb.Render(50, []string{})

	// Should not panic, should return some string (potentially empty with styling)
	// The result will have styling applied even if content is empty
	if result == "" {
		t.Log("Empty items returns empty string (which is valid)")
	}
}

func TestStatusBar_Render_NarrowWidth(t *testing.T) {
	sb := NewStatusBar()
	items := []string{"↑↓ Navigate", "Enter Select", "q Quit"}
	result := sb.Render(20, items)

	// Should still render without panicking
	// Content may be truncated or overflow depending on lipgloss behavior
	if result == "" {
		t.Error("expected non-empty result even with narrow width")
	}
}

func TestStatusBar_Render_WideWidth(t *testing.T) {
	sb := NewStatusBar()
	items := []string{"Help"}
	result := sb.Render(100, items)

	// Should render with padding to fill width
	if !strings.Contains(result, "Help") {
		t.Errorf("expected result to contain 'Help', got: %s", result)
	}
}

func TestStatusBar_Render_SpecialCharacters(t *testing.T) {
	sb := NewStatusBar()
	items := []string{"Ctrl+C Cancel", "Esc Back", "/ Filter"}
	result := sb.Render(60, items)

	// Should handle special characters
	if !strings.Contains(result, "Ctrl+C Cancel") {
		t.Errorf("expected result to contain 'Ctrl+C Cancel', got: %s", result)
	}
	if !strings.Contains(result, "Esc Back") {
		t.Errorf("expected result to contain 'Esc Back', got: %s", result)
	}
}

func TestNewStatusBar(t *testing.T) {
	sb := NewStatusBar()

	// StatusBar is a simple struct, just verify it doesn't panic
	_ = sb.Render(50, []string{"test"})
}

func TestStatusBar_Render_SeparatorFormat(t *testing.T) {
	sb := NewStatusBar()
	items := []string{"A", "B", "C"}
	result := sb.Render(40, items)

	// Check that items are joined with "  |  " separator
	if !strings.Contains(result, "A  |  B  |  C") {
		t.Errorf("expected items to be joined with '  |  ', got: %s", result)
	}
}
