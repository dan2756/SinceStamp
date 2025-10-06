package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
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

func TestShowFormats(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee")
	ta.advance(2*time.Hour + 14*time.Minute + 9*time.Second)

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"coffee", "--short"}, "coffee: 2h14m ago (Mon Jan 5 09:12)\n"},
		{[]string{"coffee", "--seconds"}, "8049\n"},
		{[]string{"-seconds", "coffee"}, "8049\n"},
	}
	for _, tt := range tests {
		if got := ta.must(tt.args...); got != tt.want {
			t.Errorf("since %v = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestShowJSON(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee", "--at", "1h", "--note", "espresso")
	ta.must("mark", "coffee", "--note", "flat white")
	ta.advance(90 * time.Second)

	var got map[string]any
	if err := json.Unmarshal([]byte(ta.must("coffee", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name":            "coffee",
		"last":            "2026-01-05T09:12:00-05:00",
		"elapsed_seconds": float64(90),
		"count":           float64(2),
		"note":            "flat white",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestListJSON(t *testing.T) {
	ta := newTestApp(t)
	if got := ta.must("ls", "--json"); got != "[]\n" {
		t.Errorf("empty list = %q, want []", got)
	}
	ta.must("mark", "gym", "--at", "2d")
	ta.must("mark", "coffee")
	ta.advance(time.Minute)

	var got []eventJSON
	if err := json.Unmarshal([]byte(ta.must("--json")), &got); err != nil {
		t.Fatal(err)
	}
	want := []eventJSON{
		{Name: "coffee", Last: "2026-01-05T09:12:00-05:00", ElapsedSeconds: 60, Count: 1},
		{Name: "gym", Last: "2026-01-03T09:12:00-05:00", ElapsedSeconds: 2*86400 + 60, Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestListSeconds(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "tea")
	ta.advance(42 * time.Second)
	if got, want := ta.must("ls", "--seconds"), "tea  42\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemove(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "a")
	ta.must("mark", "a")
	ta.must("mark", "b")
	ta.must("mark", "c")

	if got, want := ta.must("rm", "a", "b"), "removed a (2 marks)\nremoved b (1 mark)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if code, _ := ta.run("a"); code != 1 {
		t.Errorf("removed event still shows up (exit %d)", code)
	}
	// All or nothing: c survives when one of the names is unknown.
	if code, _ := ta.run("rm", "c", "nope"); code != 1 {
		t.Errorf("rm of an unknown event: exit %d, want 1", code)
	}
	ta.must("c")
}

func TestRename(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "cofee", "--at", "1h")
	ta.must("mark", "cofee")
	ta.must("mark", "tea")

	if got, want := ta.must("rename", "cofee", "coffee"), "renamed cofee to coffee\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if code, _ := ta.run("cofee"); code != 1 {
		t.Errorf("old name still resolves (exit %d)", code)
	}
	if !strings.Contains(ta.must("log", "coffee"), "coffee: 2 marks") {
		t.Error("history didn't move with the rename")
	}
	if code, _ := ta.run("rename", "coffee", "tea"); code != 1 {
		t.Errorf("rename onto an existing event: exit %d, want 1", code)
	}
	if code, _ := ta.run("rename", "nope", "x"); code != 1 {
		t.Errorf("rename of an unknown event: exit %d, want 1", code)
	}
}

func TestShowSubcommandForReservedNames(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "log")
	ta.advance(time.Hour)
	if got, want := ta.must("show", "log"), "log: 1h ago (Mon Jan 5 09:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
