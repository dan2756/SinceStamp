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

func TestMarkKeepsHistorySortedAndCapped(t *testing.T) {
	base := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	d := newData()
	for i := range maxMarks + 5 {
		d.Mark("tea", Mark{At: base.Add(time.Duration(i) * time.Minute)})
	}
	// An older mark lands in the middle, a duplicate time goes after its twin.
	d.Mark("tea", Mark{At: base.Add(50*time.Minute + 30*time.Second), Note: "late entry"})
	d.Mark("tea", Mark{At: base.Add(60 * time.Minute), Note: "twin"})

	e := d.Events["tea"]
	if e.Count != maxMarks+7 {
		t.Errorf("count = %d, want %d", e.Count, maxMarks+7)
	}
	if len(e.Marks) != maxMarks {
		t.Fatalf("kept %d marks, want %d", len(e.Marks), maxMarks)
	}
	for i := 1; i < len(e.Marks); i++ {
		if e.Marks[i].At.Before(e.Marks[i-1].At) {
			t.Fatalf("marks out of order at %d: %v before %v", i, e.Marks[i].At, e.Marks[i-1].At)
		}
	}
	if want := base.Add(7 * time.Minute); !e.Marks[0].At.Equal(want) {
		t.Errorf("oldest kept = %v, want %v", e.Marks[0].At, want)
	}
	var notes []string
	for _, m := range e.Marks {
		if m.Note != "" {
			notes = append(notes, m.Note)
		}
		if m.At.Equal(base.Add(60*time.Minute)) && m.Note == "" && len(notes) == 2 {
			t.Error("a mark with the same time should be inserted after the existing one")
		}
	}
	if !reflect.DeepEqual(notes, []string{"late entry", "twin"}) {
		t.Errorf("notes = %v", notes)
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
	for _, e := range entries {
		switch e.Name() {
		case "events.json", "events.json.lock":
		default:
			t.Errorf("unexpected file left in data dir: %s", e.Name())
		}
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

// A symlinked events file, say into a synced folder, used to be replaced by
// a regular file on the first write, so later marks never reached the
// target.
func TestUpdateThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "synced", "events.json")
	link := filepath.Join(dir, "events.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("synced", "events.json"), link); err != nil {
		t.Skipf("can't create a symlink: %v", err)
	}

	s := &Store{Path: link}
	if err := s.Update(func(d *Data) error {
		d.Mark("coffee", Mark{At: time.Now().UTC()})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link was replaced: %v, %v", fi, err)
	}
	d, err := (&Store{Path: target}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if d.Events["coffee"] == nil {
		t.Error("the mark didn't reach the link's target")
	}
	fi, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %v, want the original -rw-r--r--", got)
	}
	if _, err := os.Stat(target + ".lock"); err != nil {
		t.Errorf("lock should sit next to the target: %v", err)
	}
}

func TestSaveThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "synced", "events.json")
	link := filepath.Join(dir, "events.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("can't create a symlink: %v", err)
	}
	if err := (&Store{Path: link}).Save(newData()); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link was replaced: %v, %v", fi, err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target wasn't created: %v", err)
	}
}
