package main

import "strings"

// options are the flags understood by every subcommand. Flags may appear
// anywhere on the command line, before or after positional arguments.
type options struct {
	help    bool
	version bool
	json    bool
	short   bool
	seconds bool

	at      string
	atSet   bool
	note    string
	noteSet bool
}

// parseArgs splits args into flags and positional arguments. Both -flag and
// --flag spellings are accepted, values can be given as "--at 3h" or
// "--at=3h", and "--" ends flag parsing.
func parseArgs(args []string) (options, []string, error) {
	var o options
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			pos = append(pos, arg)
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
		name, value, hasValue := strings.Cut(name, "=")

		switch name {
		case "at", "note":
			if !hasValue {
				if i+1 >= len(args) {
					return o, nil, usagef("flag --%s needs a value", name)
				}
				i++
				value = args[i]
			}
			if name == "at" {
				o.at, o.atSet = value, true
			} else {
				o.note, o.noteSet = value, true
			}
			continue
		}

		var flag *bool
		switch name {
		case "h", "help":
			flag = &o.help
		case "version":
			flag = &o.version
		case "json":
			flag = &o.json
		case "short":
			flag = &o.short
		case "seconds":
			flag = &o.seconds
		default:
			return o, nil, usagef("unknown flag %s", arg)
		}
		if hasValue {
			return o, nil, usagef("flag --%s doesn't take a value", name)
		}
		*flag = true
	}
	return o, pos, nil
}

func (o options) style() style {
	return style{short: o.short}
}
