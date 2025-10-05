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
since mark NAME    record that NAME happened just now
since NAME         show how long ago NAME last happened
since [ls]         list all events, most recent first
```

Events are kept in `$XDG_DATA_HOME/since/events.json`
(`~/.local/share/since/events.json` by default). Set `SINCE_FILE` to use a
different file.

## License

MIT
