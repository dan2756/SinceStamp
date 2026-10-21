package main

import (
	"testing"
	"time"
)

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 1, 5, 11, 26, 0, 0, testZone)
	local := func(y int, mo time.Month, d, h, mi, s int) time.Time {
		return time.Date(y, mo, d, h, mi, s, 0, testZone)
	}
	tests := []struct {
		in   string
		want time.Time
	}{
		{"now", now},
		{"NOW", now},
		{"3h", local(2026, 1, 5, 8, 26, 0)},
		{"3h ago", local(2026, 1, 5, 8, 26, 0)},
		{"3h AGO", local(2026, 1, 5, 8, 26, 0)},
		{"3H Ago", local(2026, 1, 5, 8, 26, 0)},
		{"2 Days Ago", local(2026, 1, 3, 11, 26, 0)},
		{"2 DAYS AGO", local(2026, 1, 3, 11, 26, 0)},
		{"-3h", local(2026, 1, 5, 8, 26, 0)},
		{"90m", local(2026, 1, 5, 9, 56, 0)},
		{"1.5h", local(2026, 1, 5, 9, 56, 0)},
		{"45s", local(2026, 1, 5, 11, 25, 15)},
		{"1d2h", local(2026, 1, 4, 9, 26, 0)},
		{"1d 2h", local(2026, 1, 4, 9, 26, 0)},
		{"2 days 3 hours ago", local(2026, 1, 3, 8, 26, 0)},
		{"1h, 30 min", local(2026, 1, 5, 9, 56, 0)},
		{"1w", local(2025, 12, 29, 11, 26, 0)},
		{"2026-01-02 15:04", local(2026, 1, 2, 15, 4, 0)},
		{"2026-01-02 15:04:05", local(2026, 1, 2, 15, 4, 5)},
		{"2026-01-02T15:04", local(2026, 1, 2, 15, 4, 0)},
		{"2026-01-02", local(2026, 1, 2, 0, 0, 0)},
		{"2026-01-02T15:04:05Z", time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)},
		{"2026-01-02T15:04:05+01:00", time.Date(2026, 1, 2, 14, 4, 5, 0, time.UTC)},
		{"2026-01-02T15:04:05.5Z", time.Date(2026, 1, 2, 15, 4, 5, 5e8, time.UTC)},
		{"09:12", local(2026, 1, 5, 9, 12, 0)},
		{"9:12", local(2026, 1, 5, 9, 12, 0)},
		{"11:26", now},
		{"10:00:30", local(2026, 1, 5, 10, 0, 30)},
		// A clock time that hasn't come yet today means yesterday.
		{"23:00", local(2026, 1, 4, 23, 0, 0)},
		// Future times, for things that are coming up.
		{"+3h", local(2026, 1, 5, 14, 26, 0)},
		{"+1d2h", local(2026, 1, 6, 13, 26, 0)},
		{"in 2 days", local(2026, 1, 7, 11, 26, 0)},
		{"In 30m", local(2026, 1, 5, 11, 56, 0)},
		{"2026-03-01 10:00", local(2026, 3, 1, 10, 0, 0)},
		{"1900-01-01", local(1900, 1, 1, 0, 0, 0)},
	}
	for _, tt := range tests {
		got, err := parseWhen(tt.in, now, testZone)
		if err != nil {
			t.Errorf("parseWhen(%q): %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("parseWhen(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseWhenErrors(t *testing.T) {
	now := time.Date(2026, 1, 5, 11, 26, 0, 0, testZone)
	for _, in := range []string{
		"", "  ", "soon", "3", "3x", "3h4", "h", "1..5h", "1e9y", "300y",
		"2026-13-01", "2026-01-02 25:00", "24:00", "ago",
		"+", "in", "in ", "in 3h ago", "+-3h",
		// Absolute dates get the same 200-year limit as relative ones.
		"2400-01-01", "1700-01-01 12:00", "2400-01-01T00:00:00Z",
	} {
		if got, err := parseWhen(in, now, testZone); err == nil {
			t.Errorf("parseWhen(%q) = %v, want an error", in, got)
		}
	}
}

func TestParseSpan(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"3h", 3 * time.Hour},
		{"3H", 3 * time.Hour},
		{"1h30m", 90 * time.Minute},
		{"2d", 2 * day},
		{"1w2d", 9 * day},
		{"1y", year},
		{"0.5d", 12 * time.Hour},
		{"10 seconds", 10 * time.Second},
	}
	for _, tt := range tests {
		got, err := parseSpan(tt.in)
		if err != nil {
			t.Errorf("parseSpan(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSpan(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
