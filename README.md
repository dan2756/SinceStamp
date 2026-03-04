# since

How long has it been since you last...? `since` records when things happen and
tells you how long ago that was. Watered the plants, changed the toothbrush,
took the meds, had a coffee.

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

## Usage

```
since mark NAME [--at WHEN] [--note TEXT]   record that NAME happened
since NAME                                  how long ago NAME last happened
since show NAME                             same, for names like "log" or "rm"
since [ls]                                  all events, most recent first
since log NAME                              history of NAME with intervals
since rm NAME...                            delete events and their history
since rename OLD NEW                        rename an event
```

### Flags

| Flag          | Applies to          | Effect                                   |
|---------------|---------------------|------------------------------------------|
| `--at WHEN`   | mark                | when it happened instead of now          |
| `--note TEXT` | mark                | attach a note, shown by `log`            |
| `--json`      | show, ls, log, mark | machine-readable output                  |
| `--short`     | show, ls, log, mark | compact durations: `2h14m`               |
| `--seconds`   | show, ls, log, mark | whole seconds: `since coffee --seconds`  |

`--at` takes a time in the past, in any of these forms:

| Form                          | Meaning                                 |
|-------------------------------|-----------------------------------------|
| `3h`, `90m`, `1d2h`, `3h ago` | that long ago                           |
| `15:04`                       | today at 15:04 (yesterday if not yet)   |
| `"2026-01-02 15:04"`          | local date and time                     |
| `2026-01-02T15:04:05Z`        | RFC 3339, any offset                    |

### Examples

```
$ since mark coffee --at 40m --note "flat white"
$ since log coffee
coffee: 3 marks, last 40m ago, every 4h 2m on average
  Mon 2026-01-05 01:08
  Mon 2026-01-05 05:10  +4h 2m
  Mon 2026-01-05 09:12  +4h 2m  flat white

$ since
coffee  40m ago
plants  3d 4h ago
filter  6w 2d ago

$ since coffee --json
{
  "name": "coffee",
  "last": "2026-01-05T09:12:00-05:00",
  "elapsed_seconds": 2400,
  "count": 3,
  "note": "flat white"
}

$ [ "$(since plants --seconds)" -gt 259200 ] && echo "water the plants"
water the plants
```

`since log NAME --json` adds `average_interval_seconds` and an `entries`
array of `{at, interval_seconds, note}`.

The last 100 marks of each event are kept; `count` keeps counting past that.

## Storage

Events are kept in `$XDG_DATA_HOME/since/events.json`
(`~/.local/share/since/events.json` by default). Set `SINCE_FILE` to use a
different file.

The file is plain JSON and safe to edit by hand. Writes go to a temporary file
that is renamed into place, so it is never left half-written, and updates hold
an exclusive lock on `events.json.lock` so marks made at the same moment (from
a hook, cron, or two terminals) all get recorded.

## License

MIT
