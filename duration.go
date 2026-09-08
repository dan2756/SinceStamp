package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	day  = 24 * time.Hour
	week = 7 * day
	year = 365 * day
)

type unit struct {
	suffix string
	size   time.Duration
}

// units are ordered from largest to smallest. A year is 365 days; months are
// left out because they don't have a fixed length.
var units = []unit{
	{"y", year},
	{"w", week},
	{"d", day},
	{"h", time.Hour},
	{"m", time.Minute},
	{"s", time.Second},
}

// style controls how durations are written.
type style struct {
	short   bool // "2h14m" instead of "2h 14m"
	precise bool // every unit down to seconds: "2h 14m 9s"
}

// formatDuration renders d using its largest unit and, when non-zero, the
// unit right below it: "45s", "2h 14m", "3d", "1y 6w". With st.precise every
// non-zero unit down to seconds is included. The sign of d is ignored and
// anything below a second is dropped.
func formatDuration(d time.Duration, st style) string {
	if d < 0 {
		// Negating the smallest Duration, which is what time.Sub
		// saturates to, would overflow.
		d = -max(d, -math.MaxInt64)
	}
	var parts []string
	first := -1
	for i, u := range units {
		n := d / u.size
		d -= n * u.size
		if first < 0 {
			if n == 0 {
				continue
			}
			first = i
		} else if !st.precise && i > first+1 {
			break
		}
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", n, u.suffix))
		}
	}
	if len(parts) == 0 {
		return "0s"
	}
	sep := " "
	if st.short {
		sep = ""
	}
	return strings.Join(parts, sep)
}

// relative describes the time since something happened: "2h 14m ago",
// "just now", or "in 3h" when it is still in the future.
func relative(d time.Duration, st style) string {
	switch {
	case d > -time.Second && d < time.Second:
		return "just now"
	case d < 0:
		return "in " + formatDuration(d, st)
	default:
		return formatDuration(d, st) + " ago"
	}
}

// seconds is d in whole seconds, truncated toward zero.
func seconds(d time.Duration) int64 {
	return int64(d / time.Second)
}
