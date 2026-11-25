package main

import (
	"strings"
	"testing"

	"github.com/feds01/umbra/internal/golden"
)

func TestViewsPostgres(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{"every table, and the classification view", []string{"views", "postgres", "--config", exampleConfig}, "views_postgres.sql"},
		{"one table, replaced", []string{"views", "postgres", "--config", exampleConfig, "--replace", "contacts"}, "views_postgres_replace.sql"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := execute(t, test.args...)

			if code != 0 {
				t.Fatalf("exit code %d: %s", code, stderr)
			}

			golden.Check(t, test.golden, stdout)
		})
	}
}

func TestAFlagAfterATableIsAUsageError(t *testing.T) {
	code, stdout, stderr := execute(t, "views", "postgres", "--config", exampleConfig, "contacts", "--replace")

	if code != 2 || stdout != "" || !strings.Contains(stderr, `"--replace" comes after a table`) {
		t.Errorf("got exit code %d, %q and %q, want 2, nothing and the usage error", code, stdout, stderr)
	}
}

func TestNothingIsPrintedForATableThatIsntInTheCatalog(t *testing.T) {
	code, stdout, stderr := execute(t, "views", "postgres", "--config", exampleConfig, "contacts", "missing")

	if code != 1 || stdout != "" || !strings.Contains(stderr, "postgres missing: not in the catalog") {
		t.Errorf("got exit code %d, %q and %q, want 1, nothing and the problem", code, stdout, stderr)
	}
}

func TestTheSourceSchemaDefaultsToPublic(t *testing.T) {
	config := project(t, map[string]string{
		configName:                      "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n[postgres]\ndataset = \"postgres\"\nviews = \"masked\"\n",
		"catalog/postgres/contacts.pii": "id allow\nname deny=person\n",
	})

	code, stdout, stderr := execute(t, "views", "postgres", "--config", config)

	if code != 0 || !strings.Contains(stdout, `FROM public."contacts"`) {
		t.Errorf("got exit code %d, %q and %q, want 0 and a view of public.contacts", code, stdout, stderr)
	}
}

func TestTheNearestConfigIsFound(t *testing.T) {
	config := project(t, map[string]string{
		configName:                      "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n[postgres]\ndataset = \"postgres\"\nviews = \"masked\"\n",
		"catalog/postgres/contacts.pii": "id allow\nname deny=person\n",
		"deeper/.keep":                  "",
	})

	t.Chdir(config[:len(config)-len(configName)] + "deeper")

	code, stdout, stderr := execute(t, "views", "postgres")

	if code != 0 || !strings.Contains(stdout, `CREATE VIEW masked."contacts"`) {
		t.Errorf("got exit code %d, %q and %q", code, stdout, stderr)
	}
}
