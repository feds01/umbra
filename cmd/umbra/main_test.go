package main

import (
	"bytes"
	"errors"
	"runtime/debug"
	"strings"
	"testing"
)

// execute runs the command, and returns its exit code and what it printed.
func execute(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errs bytes.Buffer

	code = run(args, &out, &errs)

	return code, out.String(), errs.String()
}

func TestACommandThatCouldNotRunExitsWithTwo(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "usage: umbra"},
		{"an unknown command", []string{"fmt"}, `unknown command "fmt"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := execute(t, test.args...)

			if code != 2 || !strings.Contains(stderr, test.want) {
				t.Errorf("got exit code %d and %q, want 2 and %q", code, stderr, test.want)
			}
		})
	}
}

func TestHelpAndVersion(t *testing.T) {
	code, stdout, _ := execute(t, "help")

	if code != 0 || !strings.HasPrefix(stdout, "usage: umbra") {
		t.Errorf("help: got exit code %d and %q, want 0 and the usage", code, stdout)
	}

	code, stdout, _ = execute(t, "version")

	if code != 0 || !strings.HasPrefix(stdout, "umbra ") {
		t.Errorf("version: got exit code %d and %q, want 0 and a version line", code, stdout)
	}
}

func TestDescribe(t *testing.T) {
	built := func(module string, settings ...debug.BuildSetting) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: module}, Settings: settings}
	}

	checkout := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "a1b2c3d4e5f6"},
		{Key: "vcs.time", Value: "2026-11-25T14:09:56Z"},
	}

	tests := []struct {
		name    string
		version string
		info    *debug.BuildInfo
		want    string
	}{
		{"no build info", "dev", nil, "umbra dev"},
		{"a release build from a checkout", "0.1.0", built("(devel)", checkout...), "umbra 0.1.0 (a1b2c3d 2026-11-25)"},
		{"a local build from a checkout", "dev", built("(devel)", checkout...), "umbra dev (a1b2c3d 2026-11-25)"},
		{"go install of a release", "dev", built("v0.1.0"), "umbra 0.1.0"},
		{"go install of no release", "dev", built("(devel)"), "umbra dev"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := describe(test.version, test.info); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// fullDisk is a writer that always fails.
type fullDisk struct{}

func (fullDisk) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }

func TestOutputThatCouldNotBeWrittenExitsWithTwo(t *testing.T) {
	var errs bytes.Buffer

	code := run([]string{"version"}, fullDisk{}, &errs)

	if code != 2 || !strings.Contains(errs.String(), "no space left on device") {
		t.Errorf("got exit code %d and %q, want 2 and the write error", code, errs.String())
	}
}
