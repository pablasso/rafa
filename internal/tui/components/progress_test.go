package components

import (
	"strings"
	"testing"
)

func TestProgress_View_ZeroPercent(t *testing.T) {
	p := NewProgress(0, 10, 8)
	result := p.View()

	// Should show all empty: □□□□□□□□ 0%
	if !strings.HasPrefix(result, "□□□□□□□□") {
		t.Errorf("expected all empty boxes, got: %s", result)
	}
	if !strings.HasSuffix(result, "0%") {
		t.Errorf("expected 0%%, got: %s", result)
	}
}

func TestProgress_View_FiftyPercent(t *testing.T) {
	p := NewProgress(5, 10, 8)
	result := p.View()

	// Should show half filled: ■■■■□□□□ 50%
	if !strings.HasPrefix(result, "■■■■□□□□") {
		t.Errorf("expected half filled ■■■■□□□□, got: %s", result)
	}
	if !strings.HasSuffix(result, "50%") {
		t.Errorf("expected 50%%, got: %s", result)
	}
}

func TestProgress_View_HundredPercent(t *testing.T) {
	p := NewProgress(10, 10, 8)
	result := p.View()

	// Should show all filled: ■■■■■■■■ 100%
	if !strings.HasPrefix(result, "■■■■■■■■") {
		t.Errorf("expected all filled boxes, got: %s", result)
	}
	if !strings.HasSuffix(result, "100%") {
		t.Errorf("expected 100%%, got: %s", result)
	}
}

func TestProgress_View_ZeroTotal(t *testing.T) {
	p := NewProgress(5, 0, 8)
	result := p.View()

	// Should return empty string for invalid total
	if result != "" {
		t.Errorf("expected empty string for zero total, got: %s", result)
	}
}

func TestProgress_View_ZeroWidth(t *testing.T) {
	p := NewProgress(5, 10, 0)
	result := p.View()

	// Should return empty string for zero width
	if result != "" {
		t.Errorf("expected empty string for zero width, got: %s", result)
	}
}

func TestProgress_View_NegativeCurrent(t *testing.T) {
	p := NewProgress(-5, 10, 8)
	result := p.View()

	// Should clamp to 0%
	if !strings.HasPrefix(result, "□□□□□□□□") {
		t.Errorf("expected all empty for negative current, got: %s", result)
	}
	if !strings.HasSuffix(result, "0%") {
		t.Errorf("expected 0%%, got: %s", result)
	}
}

func TestProgress_View_CurrentExceedsTotal(t *testing.T) {
	p := NewProgress(15, 10, 8)
	result := p.View()

	// Should clamp to 100%
	if !strings.HasPrefix(result, "■■■■■■■■") {
		t.Errorf("expected all filled for current > total, got: %s", result)
	}
	if !strings.HasSuffix(result, "100%") {
		t.Errorf("expected 100%%, got: %s", result)
	}
}

func TestProgress_View_DifferentWidths(t *testing.T) {
	tests := []struct {
		width    int
		current  int
		total    int
		expected string
	}{
		{4, 2, 4, "■■□□ 50%"},
		{10, 3, 10, "■■■□□□□□□□ 30%"},
		{6, 1, 3, "■■□□□□ 33%"},
	}

	for _, tt := range tests {
		p := NewProgress(tt.current, tt.total, tt.width)
		result := p.View()
		if result != tt.expected {
			t.Errorf("Progress(%d, %d, %d).View() = %q, want %q",
				tt.current, tt.total, tt.width, result, tt.expected)
		}
	}
}

func TestNewProgress(t *testing.T) {
	p := NewProgress(3, 10, 20)

	if p.Current != 3 {
		t.Errorf("expected Current=3, got %d", p.Current)
	}
	if p.Total != 10 {
		t.Errorf("expected Total=10, got %d", p.Total)
	}
	if p.Width != 20 {
		t.Errorf("expected Width=20, got %d", p.Width)
	}
}
