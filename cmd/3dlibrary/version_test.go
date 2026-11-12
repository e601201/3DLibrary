package main

import "testing"

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"v0.1.0", "v0.1.0"},
		{"v0.1.0+dirty", "v0.1.0+dirty"},
		{"v0.1.1-0.20261112110442-a222302eb186", "v0.1.1-0.20261112110442-a222302eb186"},
		{"(devel)", "dev"},
		{"", "dev"},
	}
	for _, tt := range tests {
		if got := displayVersion(tt.in); got != tt.want {
			t.Errorf("displayVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
