package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testApp runs commands against a temporary events file with a clock the
// test controls.
type testApp struct {
	t      *testing.T
	app    *app
	clock  time.Time
	stdout bytes.Buffer
	stderr bytes.Buffer
}

// testZone is a fixed zone so output doesn't depend on the machine running
// the tests.
var testZone = time.FixedZone("EST", -5*60*60)

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	ta := &testApp{t: t, clock: time.Date(2026, 1, 5, 9, 12, 0, 0, testZone)}
	env := map[string]string{"SINCE_FILE": filepath.Join(t.TempDir(), "events.json")}
	ta.app = &app{
		stdout: &ta.stdout,
		stderr: &ta.stderr,
		now:    func() time.Time { return ta.clock },
		loc:    testZone,
		getenv: func(k string) string { return env[k] },
	}
	return ta
}

// run executes args and returns the exit status and stdout.
func (ta *testApp) run(args ...string) (int, string) {
	ta.t.Helper()
	ta.stdout.Reset()
	ta.stderr.Reset()
	code := ta.app.run(args)
	return code, ta.stdout.String()
}

// must runs args and fails the test on a non-zero exit.
func (ta *testApp) must(args ...string) string {
	ta.t.Helper()
	code, out := ta.run(args...)
	if code != 0 {
		ta.t.Fatalf("since %s: exit %d\nstderr: %s", strings.Join(args, " "), code, ta.stderr.String())
	}
	return out
}

func (ta *testApp) advance(d time.Duration) { ta.clock = ta.clock.Add(d) }

func TestShowAfterMark(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee")
	ta.advance(2*time.Hour + 14*time.Minute)

	got := ta.must("coffee")
	if want := "coffee: 2h 14m ago (Mon Jan 5 09:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMarkAt(t *testing.T) {
	ta := newTestApp(t)
	if got, want := ta.must("mark", "coffee", "--at", "3h"), "marked coffee (Mon Jan 5 06:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ta.must("coffee"), "coffee: 3h ago (Mon Jan 5 06:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	ta.advance(2*time.Hour + 14*time.Minute)
	got := ta.must("mark", "coffee", "--note=second cup")
	if want := "marked coffee (Mon Jan 5 11:26); previous was 5h 14m earlier\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	ta.must("mark", "coffee", "--at", "2025-12-31T23:00:00Z")
	if got, want := ta.must("coffee"), "coffee: just now (Mon Jan 5 11:26)\n"; got != want {
		t.Errorf("an older --at should not become the latest mark: got %q", got)
	}
}

func TestShowUnknownEvent(t *testing.T) {
	ta := newTestApp(t)
	code, out := ta.run("tea")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("unexpected stdout %q", out)
	}
	if !strings.Contains(ta.stderr.String(), `no event named "tea"`) {
		t.Errorf("stderr = %q", ta.stderr.String())
	}
}

func TestListSortedByMostRecent(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "gym")
	ta.advance(3 * day)
	ta.must("mark", "coffee")
	ta.advance(time.Hour)
	ta.must("mark", "water")
	ta.advance(5 * time.Minute)

	want := "water   5m ago\n" +
		"coffee  1h 5m ago\n" +
		"gym     3d 1h ago\n"
	for _, args := range [][]string{nil, {"ls"}} {
		if got := ta.must(args...); got != want {
			t.Errorf("since %v:\n%s\nwant:\n%s", args, got, want)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	ta := newTestApp(t)
	for _, args := range [][]string{
		{"mark"},
		{"mark", "a", "b"},
		{"--bogus"},
		{"ls", "extra"},
		{"mark", "x", "--at"},
		{"mark", "x", "--at", "whenever"},
		{"coffee", "--note", "hi"},
		{"--help=yes"},
	} {
		if code, _ := ta.run(args...); code != 2 {
			t.Errorf("since %v: exit %d, want 2", args, code)
		}
	}
}

func TestHelpAndVersion(t *testing.T) {
	ta := newTestApp(t)
	if out := ta.must("--help"); !strings.Contains(out, "Usage:") {
		t.Errorf("help output: %q", out)
	}
	if out := ta.must("--version"); !strings.HasPrefix(out, "since ") {
		t.Errorf("version output: %q", out)
	}
}
