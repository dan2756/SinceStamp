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

// Intervals returns the whole seconds between each mark and the one before
// it, so it has one entry fewer than Marks. The count stays exact when the
// gap is longer than a Duration (about 292 years).
func (e *Event) Intervals() []int64 {
	if len(e.Marks) < 2 {
		return nil
	}
	out := make([]int64, len(e.Marks)-1)
	for i := 1; i < len(e.Marks); i++ {
		out[i-1] = wholeSeconds(e.Marks[i].At, e.Marks[i-1].At)
	}
	return out
}

// AverageInterval is the mean whole seconds between the first and last kept
// marks. ok is false when there are fewer than two marks.
func (e *Event) AverageInterval() (secs int64, ok bool) {
	n := len(e.Marks)
	if n < 2 {
		return 0, false
	}
	later, earlier := e.Marks[n-1].At, e.Marks[0].At
	d := later.Sub(earlier)
	if later.Equal(earlier.Add(d)) {
		return seconds(d / time.Duration(n-1)), true
	}
	return wholeSeconds(later, earlier) / int64(n-1), true
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
// old or the new contents, never a partial write. When the path is a
// symlink, the file it points to is replaced rather than the link, and an
// existing file keeps its permissions.
func (s *Store) Save(d *Data) (err error) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	path, err := resolveLinks(s.Path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	// Owner-only: the events file is private, so a directory created for it
	// is too. MkdirAll leaves a directory that already exists alone.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			os.Remove(tmp)
		}
	}()
	// CreateTemp makes the file 0600, which is also what a new events file
	// gets; one that already exists keeps its mode.
	if fi, statErr := os.Stat(path); statErr == nil {
		if err = f.Chmod(fi.Mode().Perm()); err != nil {
			f.Close()
			return err
		}
	}
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
	return os.Rename(tmp, path)
}

// resolveLinks follows path for as long as it is a symbolic link, so a
// write lands in the file the link points to (one in a synced folder, say)
// instead of replacing the link. The target doesn't have to exist yet.
func resolveLinks(path string) (string, error) {
	orig := path
	for range 40 {
		target, err := os.Readlink(path)
		if err != nil {
			// Not a link, or nothing there yet: this is the file.
			return path, nil
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	return "", fmt.Errorf("%s: too many levels of symbolic links", orig)
}

// Update loads the store, applies fn and saves the result. Nothing is
// written if fn returns an error.
//
// The whole read-modify-write runs under an exclusive lock on a sidecar
// file (events.json.lock), so concurrent updates queue up instead of
// overwriting each other. Plain reads don't need the lock: Save's rename
// means they always see a complete file. The lock sits next to the file a
// symlink points to, so writers using the link and the real path share it.
func (s *Store) Update(fn func(*Data) error) error {
	path, err := resolveLinks(s.Path)
	if err != nil {
		return err
	}
	unlock, err := lockFile(path + ".lock")
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
