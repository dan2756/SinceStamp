package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEventIntervals(t *testing.T) {
	base := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	e := &Event{Marks: []Mark{
		{At: base},
		{At: base.Add(time.Hour)},
		{At: base.Add(4 * time.Hour)},
		{At: base.Add(4 * time.Hour)},
	}}
	want := []time.Duration{time.Hour, 3 * time.Hour, 0}
	if got := e.Intervals(); !reflect.DeepEqual(got, want) {
		t.Errorf("Intervals() = %v, want %v", got, want)
	}
	avg, ok := e.AverageInterval()
	if !ok || avg != 80*time.Minute {
		t.Errorf("AverageInterval() = %v, %v; want 1h20m, true", avg, ok)
	}
}

func TestEventIntervalsSingleMark(t *testing.T) {
	e := &Event{Marks: []Mark{{At: time.Now()}}}
	if got := e.Intervals(); len(got) != 0 {
		t.Errorf("Intervals() = %v, want none", got)
	}
	if _, ok := e.AverageInterval(); ok {
		t.Error("AverageInterval() should report false for a single mark")
	}
}

func TestLogCommand(t *testing.T) {
	ta := newTestApp(t)
	ta.clock = time.Date(2026, 1, 5, 11, 26, 0, 0, testZone)
	ta.must("mark", "coffee", "--at", "2026-01-05 05:10")
	ta.must("mark", "coffee", "--at=09:12", "--note", "decaf")
	// Out of order: slotted in between the other two.
	ta.must("mark", "--at", "07:00", "coffee")

	got := ta.must("log", "coffee")
	want := "coffee: 3 marks, last 2h 14m ago, every 2h 1m on average\n" +
		"  Mon 2026-01-05 05:10\n" +
		"  Mon 2026-01-05 07:00  +1h 50m\n" +
		"  Mon 2026-01-05 09:12  +2h 12m  decaf\n"
	if got != want {
		t.Errorf("log output:\n%s\nwant:\n%s", got, want)
	}
}

func TestLogSingleMark(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "gym", "--note", "legs")
	ta.advance(day + time.Hour)
	got := ta.must("log", "gym")
	want := "gym: 1 mark, last 1d 1h ago\n" +
		"  Mon 2026-01-05 09:12  legs\n"
	if got != want {
		t.Errorf("log output:\n%s\nwant:\n%s", got, want)
	}
}

func TestLogKeepsLastHundred(t *testing.T) {
	ta := newTestApp(t)
	s, err := ta.app.store()
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(d *Data) error {
		for i := range 130 {
			d.Mark("water", Mark{At: ta.clock.Add(time.Duration(i-130) * time.Hour).UTC()})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out := ta.must("log", "water")
	header, _, _ := strings.Cut(out, "\n")
	if want := "water: 130 marks (showing the last 100), last 1h ago, every 1h on average"; header != want {
		t.Errorf("header = %q, want %q", header, want)
	}
	if lines := strings.Count(out, "\n"); lines != 101 {
		t.Errorf("got %d lines, want a header and 100 marks", lines)
	}
}

func TestLogUnknownEvent(t *testing.T) {
	ta := newTestApp(t)
	if code, _ := ta.run("log", "nope"); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestLogJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.clock = time.Date(2026, 1, 5, 11, 26, 0, 0, testZone)
	ta.must("mark", "coffee", "--at", "2026-01-05 05:10")
	ta.must("mark", "coffee", "--at", "07:00")
	ta.must("mark", "coffee", "--at", "09:12", "--note", "decaf")

	var got logJSON
	if err := json.Unmarshal([]byte(ta.must("log", "coffee", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "coffee" || got.Count != 3 || got.ElapsedSeconds != 8040 || got.Last != "2026-01-05T09:12:00-05:00" {
		t.Errorf("summary = %+v", got.eventJSON)
	}
	if got.AverageIntervalSeconds == nil || *got.AverageIntervalSeconds != 7260 {
		t.Errorf("average_interval_seconds = %v, want 7260", got.AverageIntervalSeconds)
	}
	var intervals []any
	for _, e := range got.Entries {
		if e.IntervalSeconds == nil {
			intervals = append(intervals, nil)
		} else {
			intervals = append(intervals, *e.IntervalSeconds)
		}
	}
	if want := []any{nil, int64(6600), int64(7920)}; !reflect.DeepEqual(intervals, want) {
		t.Errorf("intervals = %v, want %v", intervals, want)
	}
	if got.Entries[2].Note != "decaf" || got.Entries[2].At != "2026-01-05T09:12:00-05:00" {
		t.Errorf("last entry = %+v", got.Entries[2])
	}
}

func TestLogJSONSingleMarkHasNullAverage(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "gym")
	out := ta.must("log", "gym", "--json")
	if !strings.Contains(out, `"average_interval_seconds": null`) || !strings.Contains(out, `"interval_seconds": null`) {
		t.Errorf("expected explicit nulls:\n%s", out)
	}
}

func TestLogSecondsAndShort(t *testing.T) {
	ta := newTestApp(t)
	ta.clock = time.Date(2026, 1, 5, 11, 26, 0, 0, testZone)
	ta.must("mark", "tea", "--at", "10:00")
	ta.must("mark", "tea", "--at", "11:00")

	want := "tea: 2 marks, last 26m ago, every 1h on average\n" +
		"  Mon 2026-01-05 10:00\n" +
		"  Mon 2026-01-05 11:00  +1h\n"
	if got := ta.must("log", "tea", "--short"); got != want {
		t.Errorf("--short:\n%s\nwant:\n%s", got, want)
	}
	want = "tea: 2 marks, last 1560s ago, every 3600s on average\n" +
		"  Mon 2026-01-05 10:00\n" +
		"  Mon 2026-01-05 11:00  +3600s\n"
	if got := ta.must("log", "tea", "--seconds"); got != want {
		t.Errorf("--seconds:\n%s\nwant:\n%s", got, want)
	}
}
