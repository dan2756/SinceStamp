package main

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{500 * time.Millisecond, "0s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m 30s"},
		{2*time.Hour + 14*time.Minute + 59*time.Second, "2h 14m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 30*time.Second, "2h"},
		{3*day + 4*time.Hour, "3d 4h"},
		{8 * day, "1w 1d"},
		{14*day + 5*time.Hour, "2w"},
		{year + 6*week + 2*day, "1y 6w"},
		{3*year + 2*day, "3y"},
		{-(2*time.Hour + 14*time.Minute), "2h 14m"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestRelative(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "just now"},
		{999 * time.Millisecond, "just now"},
		{time.Second, "1s ago"},
		{2*time.Hour + 14*time.Minute, "2h 14m ago"},
	}
	for _, tt := range tests {
		if got := relative(tt.d); got != tt.want {
			t.Errorf("relative(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
