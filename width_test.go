package main

import "testing"

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"coffee", 6},
		{"café", 4},
		{"e\u0301", 1}, // e + combining acute accent
		{"☕", 2},
		{"日本語", 6},
		{"ｃｏｆｆｅｅ", 12}, // fullwidth forms
		{"ｶﾌｪ", 3},     // halfwidth katakana
		{"🌱 plants", 9},
	}
	for _, tt := range tests {
		if got := displayWidth(tt.s); got != tt.want {
			t.Errorf("displayWidth(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}
