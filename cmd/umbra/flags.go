package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// flagSet is a command's flags, which always include --config.
type flagSet struct {
	*flag.FlagSet

	config string
	stderr io.Writer
}

func newFlagSet(usage string, stderr io.Writer) *flagSet {
	flags := &flagSet{FlagSet: flag.NewFlagSet("umbra "+usage, flag.ContinueOnError), stderr: stderr}

	flags.SetOutput(stderr)
	flags.StringVar(&flags.config, "config", "", "the umbra.toml to read, instead of the nearest one")

	return flags
}

// parse reads the flags. Their problem is already reported, so it is only a usage error.
func (f *flagSet) parse(args []string) error {
	err := f.Parse(args)

	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return err
	default:
		return errUsage
	}
}

// names returns the arguments after the flags, which name tables. One that looks like a
// flag is a flag in the wrong place, so it is a usage error rather than a table.
func (f *flagSet) names() ([]string, error) {
	for _, name := range f.Args() {
		if strings.HasPrefix(name, "-") {
			return nil, f.usageError("%q comes after a table, but flags come first", name)
		}
	}

	return f.Args(), nil
}

func (f *flagSet) usageError(format string, arguments ...any) error {
	fmt.Fprintf(f.stderr, "umbra: "+format+"\n", arguments...)

	return errUsage
}
