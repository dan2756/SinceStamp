package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (a *app) store() (*Store, error) {
	p, err := dataPath(a.getenv)
	if err != nil {
		return nil, err
	}
	return &Store{Path: p}, nil
}

func (a *app) load() (*Data, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.Load()
}

// when formats a timestamp for people: "Mon Jan 5 09:12", with the year
// added when it isn't the current one.
func (a *app) when(t, now time.Time) string {
	t = t.In(a.loc)
	if t.Year() != now.In(a.loc).Year() {
		return t.Format("Mon Jan 2 2006 15:04")
	}
	return t.Format("Mon Jan 2 15:04")
}

func (a *app) rfc3339(t time.Time) string {
	return t.In(a.loc).Format(time.RFC3339)
}

// dur formats a duration for prose, honouring --short and --seconds.
func (o options) dur(d time.Duration) string {
	if o.seconds {
		return strconv.FormatInt(seconds(d), 10) + "s"
	}
	return formatDuration(d, o.style())
}

// ago describes how long ago something happened, honouring the output flags.
func (o options) ago(d time.Duration) string {
	switch {
	case !o.seconds:
		return relative(d, o.style())
	case d < 0:
		return "in " + o.dur(-d)
	default:
		return o.dur(d) + " ago"
	}
}

func validName(name string) error {
	if strings.TrimSpace(name) == "" {
		return usagef("event name can't be empty")
	}
	if strings.ContainsAny(name, "\t\r\n") {
		return usagef("event name can't contain tabs or newlines")
	}
	return nil
}

func notFound(name string) error {
	return fmt.Errorf("no event named %q (see 'since ls')", name)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// eventJSON is the --json form of an event's current state.
type eventJSON struct {
	Name           string `json:"name"`
	Last           string `json:"last"`
	ElapsedSeconds int64  `json:"elapsed_seconds"`
	Count          int    `json:"count"`
	Note           string `json:"note,omitempty"`
}

type entryJSON struct {
	At              string `json:"at"`
	IntervalSeconds *int64 `json:"interval_seconds"`
	Note            string `json:"note,omitempty"`
}

type logJSON struct {
	eventJSON
	AverageIntervalSeconds *int64      `json:"average_interval_seconds"`
	Entries                []entryJSON `json:"entries"`
}

func (a *app) summary(name string, e *Event, now time.Time) eventJSON {
	last := e.Last()
	return eventJSON{
		Name:           name,
		Last:           a.rfc3339(last.At),
		ElapsedSeconds: seconds(now.Sub(last.At)),
		Count:          e.Count,
		Note:           last.Note,
	}
}

func (a *app) writeJSON(v any) error {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (a *app) cmdMark(o options, args []string) error {
	if len(args) != 1 {
		return usagef("usage: since mark NAME [--at WHEN] [--note TEXT]")
	}
	name := args[0]
	if err := validName(name); err != nil {
		return err
	}
	now := a.now()
	at := now
	if o.atSet {
		var err error
		if at, err = parseWhen(o.at, now, a.loc); err != nil {
			return usagef("--at: %v", err)
		}
	}
	s, err := a.store()
	if err != nil {
		return err
	}
	m := Mark{At: at.Truncate(time.Second).UTC(), Note: o.note}
	var prev *Mark
	var ev Event
	err = s.Update(func(d *Data) error {
		if e := d.Events[name]; e != nil {
			p := e.Last()
			prev = &p
		}
		ev = *d.Mark(name, m)
		return nil
	})
	if err != nil {
		return err
	}
	if o.json {
		// Describe the mark just recorded, as the text output does, even
		// when --at put it before the event's latest mark.
		out := a.summary(name, &ev, now)
		out.Last = a.rfc3339(m.At)
		out.ElapsedSeconds = seconds(now.Sub(m.At))
		out.Note = m.Note
		return a.writeJSON(out)
	}
	msg := fmt.Sprintf("marked %s (%s)", name, a.when(m.At, now))
	if prev != nil && m.At.After(prev.At) {
		msg += "; previous was " + o.dur(m.At.Sub(prev.At)) + " earlier"
	}
	fmt.Fprintln(a.stdout, msg)
	return nil
}

func (a *app) cmdShow(o options, args []string) error {
	if len(args) != 1 {
		return usagef("expected one event name, got %d arguments", len(args))
	}
	name := args[0]
	d, err := a.load()
	if err != nil {
		return err
	}
	e := d.Events[name]
	if e == nil {
		return notFound(name)
	}
	now := a.now()
	last := e.Last()
	elapsed := now.Sub(last.At)
	switch {
	case o.json:
		return a.writeJSON(a.summary(name, e, now))
	case o.seconds:
		fmt.Fprintln(a.stdout, seconds(elapsed))
	default:
		fmt.Fprintf(a.stdout, "%s: %s (%s)\n", name, relative(elapsed, o.style()), a.when(last.At, now))
	}
	return nil
}

// sortedNames returns event names, most recently marked first.
func sortedNames(d *Data) []string {
	names := make([]string, 0, len(d.Events))
	for name := range d.Events {
		names = append(names, name)
	}
	slices.SortFunc(names, func(x, y string) int {
		if c := d.Events[y].Last().At.Compare(d.Events[x].Last().At); c != 0 {
			return c
		}
		return strings.Compare(x, y)
	})
	return names
}

func (a *app) cmdList(o options, args []string) error {
	if len(args) != 0 {
		return usagef("ls takes no arguments")
	}
	d, err := a.load()
	if err != nil {
		return err
	}
	names := sortedNames(d)
	now := a.now()
	if o.json {
		out := make([]eventJSON, 0, len(names))
		for _, name := range names {
			out = append(out, a.summary(name, d.Events[name], now))
		}
		return a.writeJSON(out)
	}
	if len(names) == 0 {
		fmt.Fprintln(a.stderr, "no events yet; record one with: since mark NAME")
		return nil
	}
	width := 0
	for _, name := range names {
		width = max(width, displayWidth(name))
	}
	for _, name := range names {
		elapsed := now.Sub(d.Events[name].Last().At)
		value := relative(elapsed, o.style())
		if o.seconds {
			value = strconv.FormatInt(seconds(elapsed), 10)
		}
		pad := strings.Repeat(" ", width-displayWidth(name))
		fmt.Fprintf(a.stdout, "%s%s  %s\n", name, pad, value)
	}
	return nil
}

func (a *app) cmdLog(o options, args []string) error {
	if len(args) != 1 {
		return usagef("usage: since log NAME")
	}
	name := args[0]
	d, err := a.load()
	if err != nil {
		return err
	}
	e := d.Events[name]
	if e == nil {
		return notFound(name)
	}
	now := a.now()
	avg, hasAvg := e.AverageInterval()
	intervals := e.Intervals()

	if o.json {
		out := logJSON{eventJSON: a.summary(name, e, now), Entries: make([]entryJSON, len(e.Marks))}
		if hasAvg {
			s := seconds(avg)
			out.AverageIntervalSeconds = &s
		}
		for i, m := range e.Marks {
			out.Entries[i] = entryJSON{At: a.rfc3339(m.At), Note: m.Note}
			if i > 0 {
				s := seconds(intervals[i-1])
				out.Entries[i].IntervalSeconds = &s
			}
		}
		return a.writeJSON(out)
	}

	header := name + ": " + plural(e.Count, "mark")
	if e.Count > len(e.Marks) {
		header += fmt.Sprintf(" (showing the last %d)", len(e.Marks))
	}
	if elapsed := now.Sub(e.Last().At); elapsed < 0 {
		header += ", next " + o.ago(elapsed)
	} else {
		header += ", last " + o.ago(elapsed)
	}
	if hasAvg {
		header += ", every " + o.dur(avg) + " on average"
	}
	fmt.Fprintln(a.stdout, header)

	cells := make([]string, len(e.Marks))
	width := 0
	for i, iv := range intervals {
		cells[i+1] = "+" + o.dur(iv)
		width = max(width, len(cells[i+1]))
	}
	for i, m := range e.Marks {
		line := "  " + m.At.In(a.loc).Format("Mon 2006-01-02 15:04")
		if width > 0 {
			line += "  " + cells[i] + strings.Repeat(" ", width-len(cells[i]))
		}
		if m.Note != "" {
			line += "  " + m.Note
		}
		fmt.Fprintln(a.stdout, strings.TrimRight(line, " "))
	}
	return nil
}

func (a *app) cmdRemove(o options, args []string) error {
	if len(args) == 0 {
		return usagef("usage: since rm NAME...")
	}
	s, err := a.store()
	if err != nil {
		return err
	}
	removed := map[string]int{}
	err = s.Update(func(d *Data) error {
		for _, name := range args {
			if _, done := removed[name]; done {
				continue // named twice
			}
			e := d.Events[name]
			if e == nil {
				return notFound(name)
			}
			removed[name] = e.Count
			delete(d.Events, name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, name := range args {
		if n, ok := removed[name]; ok {
			fmt.Fprintf(a.stdout, "removed %s (%s)\n", name, plural(n, "mark"))
			delete(removed, name)
		}
	}
	return nil
}

func (a *app) cmdRename(o options, args []string) error {
	if len(args) != 2 {
		return usagef("usage: since rename OLD NEW")
	}
	from, to := args[0], args[1]
	if err := validName(to); err != nil {
		return err
	}
	s, err := a.store()
	if err != nil {
		return err
	}
	err = s.Update(func(d *Data) error {
		e := d.Events[from]
		if e == nil {
			return notFound(from)
		}
		if from == to {
			return nil
		}
		if _, exists := d.Events[to]; exists {
			return fmt.Errorf("an event named %q already exists", to)
		}
		d.Events[to] = e
		delete(d.Events, from)
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "renamed %s to %s\n", from, to)
	return nil
}
