//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import (
	"sync"
	"testing"
	"time"
)

// Two quick marks used to be able to read the same old file, and whichever
// rename landed second silently dropped the other mark.
func TestConcurrentUpdatesKeepEveryMark(t *testing.T) {
	s := tempStore(t)
	base := time.Date(2026, 3, 4, 20, 0, 0, 0, time.UTC)
	const n = 25

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.Update(func(d *Data) error {
				d.Mark("coffee", Mark{At: base.Add(time.Duration(i) * time.Second)})
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	e := d.Events["coffee"]
	if e == nil || e.Count != n || len(e.Marks) != n {
		t.Fatalf("got %+v, want %d marks", e, n)
	}
}

func TestLockFileIsExclusive(t *testing.T) {
	path := tempStore(t).Path + ".lock"
	unlock, err := lockFile(path)
	if err != nil {
		// The directory doesn't exist yet; lockFile shouldn't care.
		t.Fatal(err)
	}

	acquired := make(chan func())
	go func() {
		u, err := lockFile(path)
		if err != nil {
			t.Error(err)
			close(acquired)
			return
		}
		acquired <- u
	}()

	select {
	case <-acquired:
		t.Fatal("second lock acquired while the first was held")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	select {
	case u, ok := <-acquired:
		if ok {
			u()
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second lock never acquired after unlock")
	}
}
