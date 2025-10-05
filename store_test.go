package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Path: filepath.Join(t.TempDir(), "nested", "events.json")}
}

func TestStoreRoundTrip(t *testing.T) {
	s := tempStore(t)
	base := time.Date(2026, 1, 5, 9, 12, 0, 0, time.UTC)

	d := newData()
	d.Mark("coffee", Mark{At: base})
	d.Mark("coffee", Mark{At: base.Add(4 * time.Hour)})
	d.Mark("gym", Mark{At: base.Add(-48 * time.Hour)})
	if err := s.Save(d); err != nil {
		t.Fatal(err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, d) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, d)
	}
	if c := got.Events["coffee"].Count; c != 2 {
		t.Errorf("coffee count = %d, want 2", c)
	}
	if last := got.Events["coffee"].Last().At; !last.Equal(base.Add(4 * time.Hour)) {
		t.Errorf("coffee last = %v", last)
	}
}

func TestStoreLoadMissingOrEmpty(t *testing.T) {
	s := tempStore(t)
	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Events) != 0 {
		t.Fatalf("expected no events, got %v", d.Events)
	}

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err != nil {
		t.Fatalf("empty file: %v", err)
	}
}

func TestStoreLoadCorrupt(t *testing.T) {
	s := tempStore(t)
	os.MkdirAll(filepath.Dir(s.Path), 0o755)
	if err := os.WriteFile(s.Path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("expected an error for a corrupt file")
	}
}

func TestStoreNormalizesHandEdits(t *testing.T) {
	s := tempStore(t)
	os.MkdirAll(filepath.Dir(s.Path), 0o755)
	doc := `{"events": {
		"tea": {"marks": [{"at": "2026-01-05T12:00:00Z"}, {"at": "2026-01-04T12:00:00Z"}]},
		"empty": {"count": 3, "marks": []}
	}}`
	if err := os.WriteFile(s.Path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Events["empty"]; ok {
		t.Error("event without marks should be dropped")
	}
	tea := d.Events["tea"]
	if tea.Count != 2 {
		t.Errorf("tea count = %d, want 2", tea.Count)
	}
	if want := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC); !tea.Last().At.Equal(want) {
		t.Errorf("tea last = %v, want %v", tea.Last().At, want)
	}
}

func TestStoreSaveLeavesNoTempFiles(t *testing.T) {
	s := tempStore(t)
	for range 3 {
		if err := s.Update(func(d *Data) error {
			d.Mark("x", Mark{At: time.Now().UTC()})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(s.Path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "events.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("unexpected files in data dir: %v", names)
	}
}

func TestDataPath(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"override", map[string]string{"SINCE_FILE": "/tmp/x.json", "XDG_DATA_HOME": "/xdg", "HOME": "/home/u"}, "/tmp/x.json"},
		{"xdg", map[string]string{"XDG_DATA_HOME": "/xdg", "HOME": "/home/u"}, "/xdg/since/events.json"},
		{"relative xdg ignored", map[string]string{"XDG_DATA_HOME": "rel", "HOME": "/home/u"}, "/home/u/.local/share/since/events.json"},
		{"home", map[string]string{"HOME": "/home/u"}, "/home/u/.local/share/since/events.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dataPath(func(k string) string { return tt.env[k] })
			if err != nil {
				t.Fatal(err)
			}
			if got != filepath.FromSlash(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
