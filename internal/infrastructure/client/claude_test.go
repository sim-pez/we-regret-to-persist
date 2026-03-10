package client

import "testing"

func TestNormalizeCompany(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"google", "google"},
		{"Google", "google"},
		{"Meta, Inc.", "meta inc"},
		{"Y Combinator (YC)", "y combinator yc"},
		{"", ""},
		{"ACME-Corp.", "acmecorp"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeCompany(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeCompany(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
