package demo

import "testing"

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    Mode
		wantErr bool
	}{
		{name: "run", in: "run", want: ModeRun},
		{name: "create", in: "create", want: ModeCreate},
		{name: "trim and lowercase", in: "  RUN  ", want: ModeRun},
		{name: "invalid", in: "nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMode() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseMode() = %q, want %q", got, tt.want)
			}
		})
	}
}
