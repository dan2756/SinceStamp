package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Mark is a single occurrence of an event.
type Mark struct {
	At   time.Time `json:"at"`
	Note string    `json:"note,omitempty"`
}

// maxMarks is how much history is kept per event.
const maxMarks = 100

// Event is the recorded history of one named event.
type Event struct {
	// Count is the total number of marks ever recorded. Only the newest
	// maxMarks are kept, so it can be larger than len(Marks).
	Count int `json:"count"`
	// Marks holds the history, oldest first. It is never empty.
	Marks []Mark `json:"marks"`
}

// Last returns the most recent mark.
func (e *Event) Last() Mark { return e.Marks[len(e.Marks)-1] }

// Intervals returns the time between each mark and the one before it, so
// it has one entry fewer than Marks.
func (e *Event) Intervals() []time.Duration {
	var out []time.Duration
	for i := 1; i < len(e.Marks); i++ {
		out = append(out, e.Marks[i].At.Sub(e.Marks[i-1].At))
	}
	return out
}

// AverageInterval is the mean time between consecutive marks in the kept
// history. It reports false when there are fewer than two marks.
func (e *Event) AverageInterval() (time.Duration, bool) {
	n := len(e.Marks)
	if n < 2 {
		return 0, false
	}
	return e.Marks[n-1].At.Sub(e.Marks[0].At) / time.Duration(n-1), true
}

// Data is the document stored in the events file.
type Data struct {
	Version int               `json:"version"`
	Events  map[string]*Event `json:"events"`
}

const dataVersion = 1

func newData() *Data {
	return &Data{Version: dataVersion, Events: map[string]*Event{}}
}

// Mark records m under name, creating the event if needed. Marks may arrive
// out of order (with --at) and are slotted into place; once the history is
// longer than maxMarks the oldest entries are dropped.
func (d *Data) Mark(name string, m Mark) *Event {
	e := d.Events[name]
	if e == nil {
		e = &Event{}
		d.Events[name] = e
	}
	e.Count++
	i, _ := slices.BinarySearchFunc(e.Marks, m.At, func(x Mark, t time.Time) int {
		if x.At.After(t) {
			return 1
		}
		return -1
	})
	e.Marks = slices.Insert(e.Marks, i, m)
	e.trim()
	return e
}

func (e *Event) trim() {
	if extra := len(e.Marks) - maxMarks; extra > 0 {
		e.Marks = slices.Delete(e.Marks, 0, extra)
	}
}

// normalize repairs a document that may have been edited by hand: marks are
// sorted, empty events are dropped and counts never trail the history.
func (d *Data) normalize() {
	if d.Events == nil {
		d.Events = map[string]*Event{}
	}
	for name, e := range d.Events {
		if e == nil || len(e.Marks) == 0 {
			delete(d.Events, name)
			continue
		}
		slices.SortStableFunc(e.Marks, func(a, b Mark) int { return a.At.Compare(b.At) })
		e.trim()
		e.Count = max(e.Count, len(e.Marks))
	}
}

// Store reads and writes the events file.
type Store struct {
	Path string
}

// Load reads the events file. A missing or empty file is an empty store.
func (s *Store) Load() (*Data, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return newData(), nil
	}
	if err != nil {
		return nil, err
	}
	d := newData()
	if len(bytes.TrimSpace(b)) == 0 {
		return d, nil
	}
	if err := json.Unmarshal(b, d); err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.Path, err)
	}
	if d.Version > dataVersion {
		return nil, fmt.Errorf("%s was written by a newer version of since (format %d)", s.Path, d.Version)
	}
	d.Version = dataVersion
	d.normalize()
	return d, nil
}

// Save writes d atomically: the data goes to a temporary file in the same
// directory, which is then renamed over the old file. Readers see either the
// old or the new contents, never a partial write.
func (s *Store) Save(d *Data) (err error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(s.Path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

// Update loads the store, applies fn and saves the result. Nothing is
// written if fn returns an error.
//
// The whole read-modify-write runs under an exclusive lock on a sidecar
// file (events.json.lock), so concurrent updates queue up instead of
// overwriting each other. Plain reads don't need the lock: Save's rename
// means they always see a complete file.
func (s *Store) Update(fn func(*Data) error) error {
	unlock, err := lockFile(s.Path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	d, err := s.Load()
	if err != nil {
		return err
	}
	if err := fn(d); err != nil {
		return err
	}
	return s.Save(d)
}

// dataPath picks the events file: $SINCE_FILE, then
// $XDG_DATA_HOME/since/events.json, then ~/.local/share/since/events.json.
func dataPath(getenv func(string) string) (string, error) {
	if p := getenv("SINCE_FILE"); p != "" {
		return p, nil
	}
	// The XDG spec says relative paths must be ignored.
	if x := getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "since", "events.json"), nil
	}
	home := getenv("HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("can't find a home directory; set SINCE_FILE")
		}
		home = h
	}
	return filepath.Join(home, ".local", "share", "since", "events.json"), nil
}
