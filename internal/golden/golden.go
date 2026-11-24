// Package golden compares a test's output with a file in testdata.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// Check fails the test when got differs from testdata/name. With -update, it rewrites
// the file instead, for git diff to show what changed.
func Check(t *testing.T, name, got string) {
	t.Helper()

	file := filepath.Join("testdata", name)

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v; run go test -update to write it", err)
	}

	if got != string(want) {
		t.Errorf("differs from %s; run go test -update and git diff to see how:\n%s", file, got)
	}
}
