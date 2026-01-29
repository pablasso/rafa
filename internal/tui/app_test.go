package tui

import (
	"strings"
	"testing"
)

func TestModel_View_TerminalTooSmall(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		expectSmall bool
	}{
		{
			name:        "exactly minimum size",
			width:       MinTerminalWidth,
			height:      MinTerminalHeight,
			expectSmall: false,
		},
		{
			name:        "width too small",
			width:       MinTerminalWidth - 1,
			height:      MinTerminalHeight,
			expectSmall: true,
		},
		{
			name:        "height too small",
			width:       MinTerminalWidth,
			height:      MinTerminalHeight - 1,
			expectSmall: true,
		},
		{
			name:        "both dimensions too small",
			width:       MinTerminalWidth - 10,
			height:      MinTerminalHeight - 5,
			expectSmall: true,
		},
		{
			name:        "larger than minimum",
			width:       100,
			height:      50,
			expectSmall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel()
			m.width = tt.width
			m.height = tt.height
			// Set size on the home view to avoid empty view
			m.home.SetSize(tt.width, tt.height)

			view := m.View()

			if tt.expectSmall {
				if !strings.Contains(view, "Terminal too small") {
					t.Error("expected view to contain 'Terminal too small'")
				}
				if !strings.Contains(view, "Minimum:") {
					t.Error("expected view to contain 'Minimum:'")
				}
				if !strings.Contains(view, "Current:") {
					t.Error("expected view to contain 'Current:'")
				}
			} else {
				if strings.Contains(view, "Terminal too small") {
					t.Error("did not expect view to contain 'Terminal too small'")
				}
			}
		})
	}
}

func TestModel_renderTerminalTooSmall_ShowsDimensions(t *testing.T) {
	m := initialModel()
	m.width = 50
	m.height = 10

	view := m.renderTerminalTooSmall()

	// Check that both minimum and current dimensions are shown
	if !strings.Contains(view, "60x15") {
		t.Error("expected minimum dimensions 60x15 to be shown")
	}
	if !strings.Contains(view, "50x10") {
		t.Error("expected current dimensions 50x10 to be shown")
	}
}
