// Command since tracks how long it has been since something last happened.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usageText = `since - how long has it been?

Usage:
  since mark NAME [--at WHEN] [--note TEXT]
                     record that NAME happened (just now, unless --at says otherwise)
  since NAME         show how long ago NAME last happened
  since show NAME    the same, for names that clash with a subcommand
  since [ls]         list all events, most recent first
  since log NAME     show the history of NAME with the time between marks
  since rm NAME...   delete events and their history
  since rename OLD NEW
                     rename an event

Flags:
  --at WHEN          when it happened: 3h (ago), 1d2h, 15:04, "2026-01-02 15:04",
                     or RFC 3339 such as 2026-01-02T15:04:05Z
  --note TEXT        attach a note to the mark
  --json             print JSON (show, ls, log and mark)
  --short            compact durations: 2h14m
  --seconds          durations as whole seconds
  -h, --help         show this help
  --version          print the version

The last 100 marks of each event are kept.

Events are stored in $SINCE_FILE, or $XDG_DATA_HOME/since/events.json
(default ~/.local/share/since/events.json).
`

// app holds everything a command needs from the outside world, so tests can
// swap in buffers, a fixed clock and a fake environment.
type app struct {
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
	loc    *time.Location
	getenv func(string) string
}

func main() {
	a := &app{
		stdout: os.Stdout,
		stderr: os.Stderr,
		now:    time.Now,
		loc:    time.Local,
		getenv: os.Getenv,
	}
	os.Exit(a.run(os.Args[1:]))
}

// usageError reports a bad invocation; it exits with status 2.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return usageError{fmt.Sprintf(format, args...)}
}

// run executes the command line and returns the process exit status.
func (a *app) run(args []string) int {
	opts, pos, err := parseArgs(args)
	if err == nil {
		err = a.dispatch(opts, pos)
	}
	if err == nil {
		return 0
	}
	fmt.Fprintf(a.stderr, "since: %v\n", err)
	var ue usageError
	if errors.As(err, &ue) {
		fmt.Fprintln(a.stderr, "Run 'since --help' for usage.")
		return 2
	}
	return 1
}

func (a *app) dispatch(o options, pos []string) error {
	if o.help {
		fmt.Fprint(a.stdout, usageText)
		return nil
	}
	if o.version {
		fmt.Fprintln(a.stdout, "since", buildVersion())
		return nil
	}
	if len(pos) == 0 {
		pos = []string{"ls"}
	}
	cmd, rest := pos[0], pos[1:]
	if (o.atSet || o.noteSet) && cmd != "mark" {
		return usagef("--at and --note only apply to mark")
	}
	switch cmd {
	case "ls", "list":
		return a.cmdList(o, rest)
	case "mark":
		return a.cmdMark(o, rest)
	case "show":
		return a.cmdShow(o, rest)
	case "log":
		return a.cmdLog(o, rest)
	case "rm":
		return a.cmdRemove(o, rest)
	case "rename", "mv":
		return a.cmdRename(o, rest)
	case "help":
		fmt.Fprint(a.stdout, usageText)
		return nil
	default:
		return a.cmdShow(o, pos)
	}
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
