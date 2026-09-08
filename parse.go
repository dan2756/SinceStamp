package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// absoluteLayouts are tried, in order, for dates without a zone; they are
// interpreted in the local time zone.
var absoluteLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseWhen interprets the value of --at relative to now:
//
//	3h, 1d2h, "90 min", "3h ago"   that long before now
//	+3h, "in 2 days"               that long after now
//	15:04, 15:04:05                today at that time, or yesterday if that is still ahead
//	2026-01-02 15:04, 2026-01-02   a local date and time
//	2026-01-02T15:04:05Z           RFC 3339, with any offset
//	now
func parseWhen(s string, now time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}
	if strings.EqualFold(s, "now") {
		return now, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return inRange(t, now)
	}
	for _, layout := range absoluteLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return inRange(t, now)
		}
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		c, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		n := now.In(loc)
		t := time.Date(n.Year(), n.Month(), n.Day(), c.Hour(), c.Minute(), c.Second(), 0, loc)
		if t.After(n) {
			t = t.AddDate(0, 0, -1)
		}
		return t, nil
	}

	future := false
	rel := s
	switch {
	case strings.HasPrefix(rel, "+"):
		future, rel = true, rel[1:]
	case len(rel) > 3 && strings.EqualFold(rel[:3], "in "):
		future, rel = true, rel[3:]
	default:
		rel = strings.TrimPrefix(rel, "-")
		rel = strings.TrimSpace(strings.TrimSuffix(rel, "ago"))
	}
	d, err := parseSpan(rel)
	if err != nil {
		return time.Time{}, fmt.Errorf("can't understand time %q (try 3h, +2d, 15:04, \"2026-01-02 15:04\" or RFC 3339)", s)
	}
	if future {
		return now.Add(d), nil
	}
	return now.Add(-d), nil
}

var spanUnits = map[string]time.Duration{
	"s": time.Second, "sec": time.Second, "secs": time.Second, "second": time.Second, "seconds": time.Second,
	"m": time.Minute, "min": time.Minute, "mins": time.Minute, "minute": time.Minute, "minutes": time.Minute,
	"h": time.Hour, "hr": time.Hour, "hrs": time.Hour, "hour": time.Hour, "hours": time.Hour,
	"d": day, "day": day, "days": day,
	"w": week, "wk": week, "wks": week, "week": week, "weeks": week,
	"y": year, "yr": year, "yrs": year, "year": year, "years": year,
}

// maxSpan keeps relative times well inside what time.Duration can hold.
const maxSpan = 200 * year

// inRange rejects a date further from now than a relative time may reach.
// Beyond about 292 years the elapsed time no longer fits in a Duration.
func inRange(t, now time.Time) (time.Time, error) {
	if t.Before(now.Add(-maxSpan)) || t.After(now.Add(maxSpan)) {
		return time.Time{}, fmt.Errorf("%s is more than %d years from now", t.Format("2006-01-02"), maxSpan/year)
	}
	return t, nil
}

// parseSpan parses a length of time such as "3h", "1d2h", "1.5h" or
// "2 days 3 hours". Unlike time.ParseDuration it knows days, weeks and
// years, and every number needs a unit.
func parseSpan(s string) (time.Duration, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, errors.New("empty duration")
	}
	var total time.Duration
	for s != "" {
		i := 0
		for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
			i++
		}
		if i == 0 {
			return 0, fmt.Errorf("expected a number at %q", s)
		}
		n, err := strconv.ParseFloat(s[:i], 64)
		if err != nil {
			return 0, fmt.Errorf("bad number %q", s[:i])
		}
		s = strings.TrimLeft(s[i:], " ")
		j := 0
		for j < len(s) && s[j] >= 'a' && s[j] <= 'z' {
			j++
		}
		size, ok := spanUnits[s[:j]]
		if !ok {
			if j == 0 {
				return 0, fmt.Errorf("missing unit after %v", n)
			}
			return 0, fmt.Errorf("unknown unit %q", s[:j])
		}
		s = strings.TrimLeft(strings.TrimPrefix(s[j:], ","), " ")
		if n*float64(size) > float64(maxSpan-total) {
			return 0, errors.New("duration too long")
		}
		total += time.Duration(n * float64(size))
	}
	return total, nil
}
