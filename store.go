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
	At time.Time `json:"at"`
}

// Event is the recorded history of one named event.
type Event struct {
	// Count is the total number of marks ever recorded.
	Count int `json:"count"`
	// Marks holds the history, oldest first. It is never empty.
	Marks []Mark `json:"marks"`
}

// Last returns the most recent mark.
func (e *Event) Last() Mark { return e.Marks[len(e.Marks)-1] }

// Data is the document stored in the events file.
type Data struct {
	Version int               `json:"version"`
	Events  map[string]*Event `json:"events"`
}

const dataVersion = 1

func newData() *Data {
	return &Data{Version: dataVersion, Events: map[string]*Event{}}
}

// Mark records m under name, creating the event if needed.
func (d *Data) Mark(name string, m Mark) *Event {
	e := d.Events[name]
	if e == nil {
		e = &Event{}
		d.Events[name] = e
	}
	e.Count++
	e.Marks = append(e.Marks, m)
	return e
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
func (s *Store) Update(fn func(*Data) error) error {
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
