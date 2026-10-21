package main

import (
	"math"
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
		// What time.Sub saturates to for times ~292 years apart.
		{math.MaxInt64, "292y 24w"},
		{math.MinInt64, "292y 24w"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d, style{}); got != tt.want {
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
		{-999 * time.Millisecond, "just now"},
		{time.Second, "1s ago"},
		{2*time.Hour + 14*time.Minute, "2h 14m ago"},
		{-3 * time.Hour, "in 3h"},
		{-(2*day + 30*time.Minute), "in 2d"},
		{-(5*week + 3*day), "in 5w 3d"},
	}
	for _, tt := range tests {
		if got := relative(tt.d, style{}); got != tt.want {
			t.Errorf("relative(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatDurationShort(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{2*time.Hour + 14*time.Minute, "2h14m"},
		{3 * day, "3d"},
		{year + 6*week, "1y6w"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d, style{short: true}); got != tt.want {
			t.Errorf("formatDuration(%v, short) = %q, want %q", tt.d, got, tt.want)
		}
	}
	if got := relative(2*time.Hour+14*time.Minute, style{short: true}); got != "2h14m ago" {
		t.Errorf("relative short = %q", got)
	}
}

func TestSeconds(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want int64
	}{
		{0, 0},
		{1999 * time.Millisecond, 1},
		{2*time.Hour + 14*time.Minute, 8040},
		{-1500 * time.Millisecond, -1},
	}
	for _, tt := range tests {
		if got := seconds(tt.d); got != tt.want {
			t.Errorf("seconds(%v) = %d, want %d", tt.d, got, tt.want)
		}
	}
}

func TestWholeSeconds(t *testing.T) {
	a := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	b := a.Add(2*time.Hour + 14*time.Minute + 9*time.Second)
	if got, want := wholeSeconds(b, a), int64(2*3600+14*60+9); got != want {
		t.Fatalf("wholeSeconds = %d, want %d", got, want)
	}
	// Fractional seconds truncate toward zero, same as seconds(Sub).
	early := time.Unix(0, 900_000_000).UTC()
	late := time.Unix(2, 0).UTC()
	if got, want := wholeSeconds(late, early), seconds(late.Sub(early)); got != want {
		t.Fatalf("fractional wholeSeconds = %d, want %d", got, want)
	}

	farEarly := time.Date(1600, 1, 1, 0, 0, 0, 0, time.UTC)
	farLate := time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC)
	if farEarly.Add(farLate.Sub(farEarly)).Equal(farLate) {
		t.Fatal("expected Sub to saturate across 600 years")
	}
	if got, want := wholeSeconds(farLate, farEarly), farLate.Unix()-farEarly.Unix(); got != want {
		t.Fatalf("saturated wholeSeconds = %d, want %d", got, want)
	}
}

func TestFormatSeconds(t *testing.T) {
	yearSecs := int64(year / time.Second)
	weekSecs := int64(week / time.Second)
	daySecs := int64(day / time.Second)
	tests := []struct {
		secs  int64
		short bool
		prec  bool
		want  string
	}{
		{0, false, false, "0s"},
		{45, false, false, "45s"},
		{2*3600 + 14*60 + 9, false, false, "2h 14m"},
		{2*3600 + 14*60 + 9, false, true, "2h 14m 9s"},
		{2*3600 + 14*60 + 9, true, true, "2h14m9s"},
		{400 * yearSecs, false, false, "400y"},
		{400*yearSecs + 2*weekSecs + 3*daySecs + 4, false, false, "400y 2w"},
		{400*yearSecs + 2*weekSecs + 3*daySecs + 4, false, true, "400y 2w 3d 4s"},
		{400*yearSecs + 2*weekSecs + 4, true, true, "400y2w4s"},
		{-(400*yearSecs + 2*weekSecs), false, false, "400y 2w"},
	}
	for _, tt := range tests {
		if got := formatSeconds(tt.secs, style{short: tt.short, precise: tt.prec}); got != tt.want {
			t.Errorf("formatSeconds(%d, short=%v, precise=%v) = %q, want %q", tt.secs, tt.short, tt.prec, got, tt.want)
		}
	}
}

func TestFormatDurationPrecise(t *testing.T) {
	tests := []struct {
		d     time.Duration
		short bool
		want  string
	}{
		{0, false, "0s"},
		{2*time.Hour + 14*time.Minute + 9*time.Second, false, "2h 14m 9s"},
		{2*time.Hour + 9*time.Second, false, "2h 9s"},
		{2 * time.Hour, false, "2h"},
		{year + 2*week + 3*day + 4*time.Hour + 5*time.Minute + 6*time.Second, false, "1y 2w 3d 4h 5m 6s"},
		{year + 6*time.Second, false, "1y 6s"},
		{2*time.Hour + 14*time.Minute + 9*time.Second, true, "2h14m9s"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.d, style{short: tt.short, precise: true}); got != tt.want {
			t.Errorf("formatDuration(%v, precise, short=%v) = %q, want %q", tt.d, tt.short, got, tt.want)
		}
	}
	if got := relative(-(3*time.Hour + 5*time.Second), style{precise: true}); got != "in 3h 5s" {
		t.Errorf("relative precise future = %q", got)
	}
}
