# since

How long has it been since you last...? `since` records when things happen and
tells you how long ago that was.

Watered the plants, changed the water filter, took the meds, had a coffee: mark
it once, ask later. Every read command has `--json`, so it also slots into
scripts, status bars and shell prompts.

```
$ since mark coffee
marked coffee (Mon Jan 5 09:12)
$ since coffee
coffee: 2h 14m ago (Mon Jan 5 09:12)
```

## Install

```
go install github.com/dan2756/since@latest
```

Requires Go 1.24 or newer. The result is a single `since` binary with no
runtime dependencies.

## Usage

```
since mark NAME [--at WHEN] [--note TEXT]   record that NAME happened
since NAME                                  how long ago NAME last happened
since show NAME                             same, for names like "log" or "rm"
since [ls]                                  all events, most recent first
since log NAME                              history of NAME with intervals
since rm NAME...                            delete events and their history
since rename OLD NEW                        rename an event (alias: mv)
```

Flags can go anywhere on the command line: `since --json coffee` and
`since coffee --json` are the same. Use `--` to stop flag parsing.

### Flags

| Flag          | Applies to          | Effect                                       |
|---------------|---------------------|----------------------------------------------|
| `--at WHEN`   | mark                | when it happened, instead of now             |
| `--note TEXT` | mark                | attach a note, shown by `log`                |
| `--json`      | show, ls, log, mark | machine-readable output                      |
| `--short`     | show, ls, log, mark | compact durations: `2h14m`                   |
| `--precise`   | show, ls, log, mark | every unit down to seconds: `2h 14m 9s`      |
| `--seconds`   | show, ls, log, mark | whole seconds; `since NAME --seconds` prints just the number |
| `-h, --help`  |                     | usage                                        |
| `--version`   |                     | print the version                            |

### Times for `--at`

| Form                               | Meaning                                    |
|------------------------------------|--------------------------------------------|
| `3h`, `90m`, `1d2h`, `"2 days ago"`| that long ago                              |
| `+3h`, `"in 2 days"`               | that long from now                         |
| `15:04`                            | today at 15:04, or yesterday if it's not 15:04 yet |
| `"2026-01-02 15:04"`, `2026-01-02` | local date (and time)                      |
| `2026-01-02T15:04:05Z`             | RFC 3339, any offset                       |
| `now`                              | now (the default)                          |

Units are `s`, `m`, `h`, `d`, `w` and `y` (365 days), or spelled out (`min`,
`hours`, `weeks`, ...). Decimals work: `1.5h`. Times more than 200 years
before or after now are rejected.

### How durations read

By default you get the largest unit plus the next one down, when it isn't zero:

| Elapsed              | Default    | `--short` | `--precise`        |
|----------------------|------------|-----------|--------------------|
| 45 seconds           | `45s`      | `45s`     | `45s`              |
| 2:14:09              | `2h 14m`   | `2h14m`   | `2h 14m 9s`        |
| 3 days 4 hours       | `3d 4h`    | `3d4h`    | `3d 4h`            |
| 44 days              | `6w 2d`    | `6w2d`    | `6w 2d`            |
| 411 days 2 hours     | `1y 6w`    | `1y6w`    | `1y 6w 4d 2h`      |

Less than a second reads as `just now`. A mark in the future reads as
`in 3h`, and its `elapsed_seconds` is negative.

## Examples

Backdate a mark and attach a note:

```
$ since mark coffee --at 40m --note "flat white"
marked coffee (Mon Jan 5 09:12); previous was 4h 2m earlier
```

See the rhythm of an event:

```
$ since log coffee
coffee: 3 marks, last 40m ago, every 4h 2m on average
  Mon 2026-01-05 01:08
  Mon 2026-01-05 05:10  +4h 2m
  Mon 2026-01-05 09:12  +4h 2m  flat white
```

Everything at a glance:

```
$ since
dentist  in 2d 3h
coffee   40m ago
plants   3d 4h ago
filter   6w 2d ago
```

Nag yourself from a script or your shell rc:

```
$ [ "$(since plants --seconds)" -gt 259200 ] && echo "water the plants"
water the plants
```

### JSON

```
$ since coffee --json
{
  "name": "coffee",
  "last": "2026-01-05T09:12:00-05:00",
  "elapsed_seconds": 2400,
  "count": 3,
  "note": "flat white"
}
```

`since ls --json` prints an array of these, most recent first (`[]` when there
are no events). `since log NAME --json` adds the history:

```
{
  "name": "coffee",
  ...
  "average_interval_seconds": 14520,
  "entries": [
    { "at": "2026-01-05T01:08:00-05:00", "interval_seconds": null },
    { "at": "2026-01-05T05:10:00-05:00", "interval_seconds": 14520 },
    { "at": "2026-01-05T09:12:00-05:00", "interval_seconds": 14520, "note": "flat white" }
  ]
}
```

`since mark NAME --json` prints the same fields for the mark it just
recorded, so a backdated mark shows its own time and note.

`last` and `at` are RFC 3339 in your local time zone. `count` is every mark
ever recorded; the history keeps the last 100 marks per event.

## Exit codes

| Code | Meaning                                                        |
|------|----------------------------------------------------------------|
| 0    | success                                                        |
| 1    | the event doesn't exist, the target of a rename is taken, reading/writing the events file failed, or the output couldn't be written |
| 2    | usage error: unknown flag, missing argument, unparseable `--at` |

## Storage

| Variable        | Effect                                                        |
|-----------------|---------------------------------------------------------------|
| `SINCE_FILE`    | path of the events file; overrides everything else            |
| `XDG_DATA_HOME` | events go in `$XDG_DATA_HOME/since/events.json` (absolute paths only) |
| `TZ`            | time zone used for display and for local `--at` times          |

Without either variable the file is `~/.local/share/since/events.json`.

The file is plain JSON and safe to edit by hand; out-of-order marks are sorted
when it's read. Writes go to a temporary file that is renamed into place, so it
is never left half-written, and updates hold an exclusive lock on
`events.json.lock` so marks made at the same moment (from a hook, cron, or two
terminals) all get recorded. A new file is readable only by you; an existing
one keeps its permissions.

The events file can be a symlink, for example into a synced folder: writes
replace the file it points to, and the temporary file and the lock live next
to that file. A hard link doesn't survive the rename, so use a symlink.

## Platform notes

`since` works on Linux and macOS. On macOS the default location is also
`~/.local/share/since`, not `~/Library`; set `SINCE_FILE` or `XDG_DATA_HOME`
if you'd rather keep it elsewhere (a synced folder works well).

File locking uses `flock(2)`, available on Linux, macOS and the BSDs. On
other systems the lock is skipped; writes are still atomic, but two marks
racing each other can lose one.

## License

MIT
