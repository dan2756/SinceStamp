package main

import (
	"fmt"
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

// formatDuration renders d using its largest unit and, when non-zero, the
// unit right below it: "45s", "2h 14m", "3d", "1y 6w". The sign of d is
// ignored and anything below a second is dropped.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
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
		} else if i > first+1 {
			break
		}
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", n, u.suffix))
		}
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

// relative describes an elapsed duration: "2h 14m ago", or "just now".
func relative(d time.Duration) string {
	if d < time.Second {
		return "just now"
	}
	return formatDuration(d) + " ago"
}
