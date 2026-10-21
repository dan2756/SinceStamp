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

// wholeSeconds is the whole seconds from earlier to later. Time.Sub saturates
// at about 292 years, but --at allows a mark 200 years either side of now, so
// the gap between those two has to be counted from the unix times instead.
func wholeSeconds(later, earlier time.Time) int64 {
	d := later.Sub(earlier)
	if later.Equal(earlier.Add(d)) {
		return seconds(d)
	}
	return diffSeconds(later.Unix(), earlier.Unix())
}

// diffSeconds is a-b, saturating at the edges of int64.
func diffSeconds(a, b int64) int64 {
	if a >= 0 && b < 0 && a > math.MaxInt64+b {
		return math.MaxInt64
	}
	if a < 0 && b > 0 && a < math.MinInt64+b {
		return math.MinInt64
	}
	return a - b
}

// formatSeconds renders a length in whole seconds the way formatDuration does.
// secs may be larger than a Duration can hold. The sign of secs is ignored.
func formatSeconds(secs int64, st style) string {
	if secs < 0 {
		if secs == math.MinInt64 {
			secs = math.MaxInt64
		} else {
			secs = -secs
		}
	}
	const maxSecs = int64(math.MaxInt64 / int64(time.Second))
	if secs <= maxSecs {
		return formatDuration(time.Duration(secs)*time.Second, st)
	}
	yearSecs := int64(year / time.Second)
	years := secs / yearSecs
	rem := secs % yearSecs
	if !st.precise {
		weeks := rem / int64(week/time.Second)
		if weeks == 0 {
			return fmt.Sprintf("%dy", years)
		}
		if st.short {
			return fmt.Sprintf("%dy%dw", years, weeks)
		}
		return fmt.Sprintf("%dy %dw", years, weeks)
	}
	rest := formatDuration(time.Duration(rem)*time.Second, st)
	if rest == "0s" {
		return fmt.Sprintf("%dy", years)
	}
	sep := " "
	if st.short {
		sep = ""
	}
	return fmt.Sprintf("%dy%s%s", years, sep, rest)
}
