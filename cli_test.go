package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
	// A name given twice (from a glob or xargs) is removed once.
	if got, want := ta.must("rm", "c", "c"), "removed c (1 mark)\n"; got != want {
		t.Errorf("rm c c: got %q, want %q", got, want)
	}
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

func TestFutureEvents(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee")
	ta.must("mark", "dentist", "--at", "+2d3h")
	ta.advance(time.Hour)

	if got, want := ta.must("dentist"), "dentist: in 2d 2h (Wed Jan 7 12:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ta.must("dentist", "--seconds"), "-180000\n"; got != want {
		t.Errorf("--seconds: got %q, want %q", got, want)
	}
	if got, want := ta.must("ls"), "dentist  in 2d 2h\ncoffee   1h ago\n"; got != want {
		t.Errorf("ls: got %q, want %q", got, want)
	}
	if got := ta.must("log", "dentist"); !strings.HasPrefix(got, "dentist: 1 mark, next in 2d 2h\n") {
		t.Errorf("log: got %q", got)
	}

	var ev eventJSON
	if err := json.Unmarshal([]byte(ta.must("dentist", "--json")), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.ElapsedSeconds != -180000 {
		t.Errorf("elapsed_seconds = %d, want -180000", ev.ElapsedSeconds)
	}
}

func TestMarkDSTGapIsUsageError(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	ta := newTestApp(t)
	ta.app.loc = loc
	ta.clock = time.Date(2026, 3, 8, 10, 0, 0, 0, loc)

	code, out := ta.run("mark", "coffee", "--at", "02:30")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr %s", code, ta.stderr.String())
	}
	if out != "" || !strings.Contains(ta.stderr.String(), "doesn't exist") {
		t.Fatalf("stdout %q stderr %s", out, ta.stderr.String())
	}
	if code, _ := ta.run("coffee"); code != 1 {
		t.Fatal("a skipped clock time was recorded")
	}
}

func TestShowPrecise(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee")
	ta.advance(2*time.Hour + 14*time.Minute + 9*time.Second)
	if got, want := ta.must("coffee", "--precise"), "coffee: 2h 14m 9s ago (Mon Jan 5 09:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := ta.must("coffee", "--precise", "--short"), "coffee: 2h14m9s ago (Mon Jan 5 09:12)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShowOlderThanAYear(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "passport", "--at", "2024-11-20 14:00")
	if got, want := ta.must("passport"), "passport: 1y 6w ago (Wed Nov 20 2024 14:00)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// mark --json used to describe the event's newest mark, so a backdated
// mark's time and note never showed up.
func TestMarkJSONDescribesNewMark(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee", "--note", "latest")
	ta.advance(time.Minute)

	var got eventJSON
	if err := json.Unmarshal([]byte(ta.must("mark", "coffee", "--at", "2026-01-02", "--note", "old", "--json")), &got); err != nil {
		t.Fatal(err)
	}
	want := eventJSON{Name: "coffee", Last: "2026-01-02T00:00:00-05:00", ElapsedSeconds: 3*86400 + 9*3600 + 13*60, Count: 2, Note: "old"}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }

// Text output used to ignore write errors and exit 0, unlike --json.
func TestStdoutWriteErrorFails(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "coffee")
	ta.app.stdout = failWriter{}
	for _, args := range [][]string{{"ls"}, {"coffee"}, {"coffee", "--json"}, {"log", "coffee"}, {"mark", "tea"}} {
		if code, _ := ta.run(args...); code != 1 {
			t.Errorf("since %v: exit %d, want 1", args, code)
		}
		if !strings.Contains(ta.stderr.String(), "no space left on device") {
			t.Errorf("since %v: stderr = %q", args, ta.stderr.String())
		}
	}
}

func TestListAlignsWideNames(t *testing.T) {
	ta := newTestApp(t)
	ta.must("mark", "x", "--at", "2h")
	ta.must("mark", "日本語", "--at", "1h")
	ta.must("mark", "e\u0301")
	ta.must("mark", "☕")

	want := "e\u0301       just now\n" +
		"☕      just now\n" +
		"日本語  1h ago\n" +
		"x       2h ago\n"
	if got := ta.must("ls"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
