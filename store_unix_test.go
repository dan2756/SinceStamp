//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestNewDataDirIsPrivate(t *testing.T) {
	// Pin the umask so 0700 is what actually lands on disk.
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	t.Run("save", func(t *testing.T) {
		s := tempStore(t)
		if err := s.Save(newData()); err != nil {
			t.Fatal(err)
		}
		assertOwnerOnlyDir(t, filepath.Dir(s.Path))
	})
	t.Run("update", func(t *testing.T) {
		s := tempStore(t)
		if err := s.Update(func(d *Data) error {
			d.Mark("coffee", Mark{At: time.Unix(0, 0).UTC()})
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		assertOwnerOnlyDir(t, filepath.Dir(s.Path))
	})
}

func TestExistingDataDirKeepsMode(t *testing.T) {
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	dir := filepath.Join(t.TempDir(), "keep")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &Store{Path: filepath.Join(dir, "events.json")}
	if err := s.Update(func(d *Data) error { return nil }); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o755 {
		t.Errorf("existing dir mode = %o, want 755", got)
	}
}

func assertOwnerOnlyDir(t *testing.T, dir string) {
	t.Helper()
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o700 {
		t.Errorf("dir mode = %o, want 700", got)
	}
}
