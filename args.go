package main

import "strings"

// options are the flags understood by every subcommand. Flags may appear
// anywhere on the command line, before or after positional arguments.
type options struct {
	help    bool
	version bool
}

// parseArgs splits args into flags and positional arguments. Both -flag and
// --flag spellings are accepted; "--" ends flag parsing.
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
		switch name {
		case "h", "help":
			o.help = true
		case "version":
			o.version = true
		default:
			return o, nil, usagef("unknown flag %s", arg)
		}
	}
	return o, pos, nil
}
