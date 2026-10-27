package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// errSpanTooLong is returned by parseSpan when a relative time is past the
// 200-year limit. parseWhen reports it on its own; every other parse error
// is a syntax problem and gets the "can't understand" hint.
var errSpanTooLong = errors.New("duration too long")

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
//
// A local time the zone skips, such as 02:30 on a spring-forward morning, is
// an error. Shifting it to a nearby hour would record a different instant
// from the one that was typed.
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
		nominal, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		t, err := time.ParseInLocation(layout, s, loc)
		if err != nil {
			continue
		}
		if !sameCivil(nominal, t) {
			return time.Time{}, fmt.Errorf("%s doesn't exist in %s", s, loc)
		}
		return inRange(t, now)
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		c, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		n := now.In(loc)
		t, ok := civilTime(n.Year(), n.Month(), n.Day(), c.Hour(), c.Minute(), c.Second(), loc)
		if !ok {
			return time.Time{}, missingCivil(s, n.Year(), n.Month(), n.Day(), loc)
		}
		if t.After(n) {
			// Previous calendar date, independent of the zone's offset.
			prev := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
			t, ok = civilTime(prev.Year(), prev.Month(), prev.Day(), c.Hour(), c.Minute(), c.Second(), loc)
			if !ok {
				return time.Time{}, missingCivil(s, prev.Year(), prev.Month(), prev.Day(), loc)
			}
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
		rel = trimAgo(rel)
	}
	d, err := parseSpan(rel)
	if errors.Is(err, errSpanTooLong) {
		return time.Time{}, fmt.Errorf("%s is more than %d years from now", s, maxSpan/year)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("can't understand time %q (try 3h, +2d, 15:04, \"2026-01-02 15:04\" or RFC 3339)", s)
	}
	if future {
		return now.Add(d), nil
	}
	return now.Add(-d), nil
}

// sameCivil reports whether local has the calendar fields parse read.
// A daylight-saving gap fails the check: the parsed time is a different hour
// (or day) from the one in the input.
func sameCivil(nominal, local time.Time) bool {
	return nominal.Year() == local.Year() && nominal.Month() == local.Month() && nominal.Day() == local.Day() &&
		nominal.Hour() == local.Hour() && nominal.Minute() == local.Minute() && nominal.Second() == local.Second()
}

// civilTime builds a local time and reports whether that clock time exists.
// time.Date normalizes a skipped hour to a different one and still succeeds.
func civilTime(year int, month time.Month, day, hour, min, sec int, loc *time.Location) (time.Time, bool) {
	t := time.Date(year, month, day, hour, min, sec, 0, loc)
	if t.Year() != year || t.Month() != month || t.Day() != day || t.Hour() != hour || t.Minute() != min || t.Second() != sec {
		return time.Time{}, false
	}
	return t, true
}

func missingCivil(input string, year int, month time.Month, day int, loc *time.Location) error {
	return fmt.Errorf("%s doesn't exist on %04d-%02d-%02d in %s", input, year, int(month), day, loc)
}

// trimAgo drops a trailing "ago" in any case. The rest of parsing is
// case-insensitive ("3H", "In 2 days", "NOW"), but a plain TrimSuffix was
// not, so "3h AGO" and "2 Days Ago" failed while "3h ago" worked.
func trimAgo(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 3 && strings.EqualFold(s[len(s)-3:], "ago") {
		s = strings.TrimSpace(s[:len(s)-3])
	}
	return s
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
			return 0, errSpanTooLong
		}
		total += time.Duration(n * float64(size))
	}
	return total, nil
}
