package main

import "testing"

func TestNormalizeAPIPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "frontend api prefix", path: "/api/auth/login", want: "/auth/login"},
		{name: "unprefixed gateway route", path: "/auth/login", want: "/auth/login"},
		{name: "api prefix is segment-aware", path: "/apix/auth/login", want: "/apix/auth/login"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAPIPath(tt.path); got != tt.want {
				t.Errorf("normalizeAPIPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
