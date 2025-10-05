# since

How long has it been since you last...? `since` records when things happen and
tells you how long ago that was.

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
since [ls]                                  all events, most recent first
since log NAME                              history of NAME with intervals
```

`--at` takes a time in the past, in any of these forms:

| Form                          | Meaning                                 |
|-------------------------------|-----------------------------------------|
| `3h`, `90m`, `1d2h`, `3h ago` | that long ago                           |
| `15:04`                       | today at 15:04 (yesterday if not yet)   |
| `"2026-01-02 15:04"`          | local date and time                     |
| `2026-01-02T15:04:05Z`        | RFC 3339, any offset                    |

```
$ since mark coffee --at 40m --note "flat white"
$ since log coffee
coffee: 3 marks, last 40m ago, every 4h 2m on average
  Mon 2026-01-05 01:08
  Mon 2026-01-05 05:10  +4h 2m
  Mon 2026-01-05 09:12  +4h 2m  flat white
```

The last 100 marks of each event are kept.

Events are kept in `$XDG_DATA_HOME/since/events.json`
(`~/.local/share/since/events.json` by default). Set `SINCE_FILE` to use a
different file.

## License

MIT
