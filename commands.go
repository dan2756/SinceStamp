package main

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
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

func (a *app) cmdMark(o options, args []string) error {
	if len(args) != 1 {
		return usagef("usage: since mark NAME")
	}
	name := args[0]
	if err := validName(name); err != nil {
		return err
	}
	s, err := a.store()
	if err != nil {
		return err
	}
	now := a.now()
	m := Mark{At: now.Truncate(time.Second).UTC()}
	var prev *Mark
	err = s.Update(func(d *Data) error {
		if e := d.Events[name]; e != nil {
			p := e.Last()
			prev = &p
		}
		d.Mark(name, m)
		return nil
	})
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("marked %s (%s)", name, a.when(m.At, now))
	if prev != nil && m.At.After(prev.At) {
		msg += "; previous was " + formatDuration(m.At.Sub(prev.At)) + " earlier"
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
	fmt.Fprintf(a.stdout, "%s: %s (%s)\n", name, relative(now.Sub(last.At)), a.when(last.At, now))
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
	if len(names) == 0 {
		fmt.Fprintln(a.stderr, "no events yet; record one with: since mark NAME")
		return nil
	}
	now := a.now()
	width := 0
	for _, name := range names {
		width = max(width, utf8.RuneCountInString(name))
	}
	for _, name := range names {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(name))
		fmt.Fprintf(a.stdout, "%s%s  %s\n", name, pad, relative(now.Sub(d.Events[name].Last().At)))
	}
	return nil
}
