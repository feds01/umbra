// Command umbra checks a catalog of classified columns against the databases it describes,
// and renders the masked views that hide what the catalog denies, without writing any Go.
//
// Run umbra help for the commands.
//
// It exits 0 when it found nothing wrong, 1 when the catalog has problems, and 2 when
// it couldn't run: a bad command line or umbra.toml, a database it couldn't reach, or
// output it couldn't write.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
)

// version is set at release time with -ldflags "-X main.version=…".
var version = "dev"

const usage = `usage: umbra <command> [flags]

  version          print the version
`

// problem is something wrong with the catalog, as opposed to a command that couldn't run.
type problem struct{ error }

func (p problem) Unwrap() error { return p.error }

// errUsage marks a command line that was already reported.
var errUsage = errors.New("usage")

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run runs a command, and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	out := &recorder{writer: stdout}

	err := dispatch(args, out, stderr)

	// A migration that was redirected to a full disk must not look complete.
	if err == nil && out.err != nil {
		err = fmt.Errorf("write the output: %w", out.err)
	}

	var found problem

	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errUsage):
		return 2
	case errors.As(err, &found):
		fmt.Fprintln(stderr, err)

		return 1
	}

	fmt.Fprintln(stderr, "umbra:", err)

	return 2
}

func dispatch(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)

		return errUsage
	}

	switch command := args[0]; command {
	case "version":
		fmt.Fprintln(stdout, describe(version, buildInfo()))

		return nil
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)

		return nil
	default:
		return fmt.Errorf("unknown command %q, see umbra help", command)
	}
}

// recorder remembers the first error that writing to its writer returned.
type recorder struct {
	writer io.Writer
	err    error
}

func (r *recorder) Write(data []byte) (int, error) {
	written, err := r.writer.Write(data)
	if err != nil && r.err == nil {
		r.err = err
	}

	return written, err
}

// buildInfo is what the Go toolchain recorded about this binary, or nil.
func buildInfo() *debug.BuildInfo {
	info, isBuilt := debug.ReadBuildInfo()
	if !isBuilt {
		return nil
	}

	return info
}

// describe is the version line, such as "umbra 0.1.0 (a1b2c3d 2026-11-25)": the commit
// and its date are those of the checkout that built it. A build from the module proxy,
// by go install, has no checkout, so it takes the module's version.
func describe(version string, info *debug.BuildInfo) string {
	if info == nil {
		return "umbra " + version
	}

	var revision, date string

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value[:min(len(setting.Value), 7)]
		case "vcs.time":
			date = setting.Value[:min(len(setting.Value), 10)]
		}
	}

	if revision == "" {
		if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}

		return "umbra " + version
	}

	return fmt.Sprintf("umbra %s (%s %s)", version, revision, date)
}
