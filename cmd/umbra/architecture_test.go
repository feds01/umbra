package main

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// zenith.toml keeps the targets of the command apart from each other, but it sees the
// library as one module from here. So this holds each target to the one library package
// that renders for it.
func TestEachTargetUsesOnlyItsOwnLibraryPackage(t *testing.T) {
	library := map[string]string{
		"postgres": "github.com/feds01/umbra/postgres",
		"bigquery": "github.com/feds01/umbra/bigquery",
	}

	for target := range library {
		files, err := filepath.Glob(filepath.Join("internal", target, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no files for the %s target: %v", target, err)
		}

		for _, file := range files {
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}

			for _, imported := range parsed.Imports {
				path, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatal(err)
				}

				for other, otherPath := range library {
					if other != target && path == otherPath {
						t.Errorf("%s imports %s, the library package of the %s target", file, path, other)
					}
				}
			}
		}
	}
}
