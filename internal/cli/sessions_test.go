package cli

import (
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "just now for less than a minute",
			duration: 30 * time.Second,
			want:     "just now",
		},
		{
			name:     "just now for 0 seconds",
			duration: 0,
			want:     "just now",
		},
		{
			name:     "1 minute ago",
			duration: 1 * time.Minute,
			want:     "1m ago",
		},
		{
			name:     "5 minutes ago",
			duration: 5 * time.Minute,
			want:     "5m ago",
		},
		{
			name:     "59 minutes ago",
			duration: 59 * time.Minute,
			want:     "59m ago",
		},
		{
			name:     "1 hour ago",
			duration: 1 * time.Hour,
			want:     "1h ago",
		},
		{
			name:     "5 hours ago",
			duration: 5 * time.Hour,
			want:     "5h ago",
		},
		{
			name:     "23 hours ago",
			duration: 23 * time.Hour,
			want:     "23h ago",
		},
		{
			name:     "1 day ago",
			duration: 24 * time.Hour,
			want:     "1d ago",
		},
		{
			name:     "3 days ago",
			duration: 72 * time.Hour,
			want:     "3d ago",
		},
		{
			name:     "10 days ago",
			duration: 240 * time.Hour,
			want:     "10d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate the time in the past
			pastTime := time.Now().Add(-tt.duration)
			got := formatAge(pastTime)
			if got != tt.want {
				t.Errorf("formatAge() = %q, want %q", got, tt.want)
			}
		})
	}
}
