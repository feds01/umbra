package main

import (
	"strings"
	"testing"

	"github.com/feds01/umbra/internal/golden"

	"github.com/feds01/umbra/cmd/umbra/internal/cli"
)

// bigquerySection is a [bigquery] section for a catalog of the one table, events.
const bigquerySection = "[bigquery]\ndataset = \"warehouse\"\nproject = \"acme\"\ntables = \"events\"\nviews = \"masked\"\nschemas = \"schemas\"\n"

// warehouse is a project of the one table, events, and the files it is given.
func warehouse(t *testing.T, schema string, files map[string]string) string {
	t.Helper()

	files[cli.ConfigName] = "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n" + bigquerySection
	files["catalog/warehouse/events.pii"] = "id allow\nname deny=person\n"

	if schema != "" {
		files["schemas/events.json"] = schema
	}

	return project(t, files)
}

func TestViewsBigQuery(t *testing.T) {
	code, stdout, stderr := execute(t, "views", "bigquery", "--config", exampleConfig)

	if code != 0 {
		t.Fatalf("exit code %d: %s", code, stderr)
	}

	golden.Check(t, "views_bigquery.sql", stdout)
}

func TestViewsBigQueryOfOneTable(t *testing.T) {
	code, stdout, stderr := execute(t, "views", "bigquery", "--config", exampleConfig, "sessions")

	if code != 0 || strings.Contains(stdout, "`acme.masked.events`") || !strings.Contains(stdout, "`acme.masked.sessions`") {
		t.Errorf("got exit code %d, %q and %q, want only the view of sessions", code, stdout, stderr)
	}
}

func TestViewsBigQueryPrintsNothingUnlessItRendersEveryTable(t *testing.T) {
	// events has its schema file, and sessions, which sorts after it, doesn't.
	config := project(t, map[string]string{
		cli.ConfigName:                   "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n" + bigquerySection,
		"catalog/warehouse/events.pii":   "id allow\n",
		"catalog/warehouse/sessions.pii": "id allow\n",
		"schemas/events.json":            `[{"name": "id", "type": "STRING"}]`,
	})

	code, stdout, stderr := execute(t, "views", "bigquery", "--config", config)

	if code != 2 || stdout != "" || !strings.Contains(stderr, "bq show --schema --format=prettyjson acme:events.sessions") {
		t.Errorf("got exit code %d, %q and %q, want 2, nothing and the bq command that writes the file", code, stdout, stderr)
	}
}

func TestAnEmptySchemaIsAnErrorNotAViewToDrop(t *testing.T) {
	config := warehouse(t, "[]", map[string]string{})

	code, stdout, stderr := execute(t, "views", "bigquery", "--config", config)

	if code != 2 || stdout != "" || !strings.Contains(stderr, "no fields") {
		t.Errorf("views: got exit code %d, %q and %q, want 2, nothing and the empty schema", code, stdout, stderr)
	}

	code, _, stderr = execute(t, "check", "--config", config)

	if code != 2 || !strings.Contains(stderr, "no fields") {
		t.Errorf("check: got exit code %d and %q, want 2 and the empty schema", code, stderr)
	}
}

func TestACatalogTableThatIsntInTheDatasetIsAProblem(t *testing.T) {
	code, stdout, stderr := execute(t, "views", "bigquery", "--config", exampleConfig, "missing")

	if code != 1 || stdout != "" || !strings.Contains(stderr, "warehouse missing: not in the catalog") {
		t.Errorf("got exit code %d, %q and %q, want 1, nothing and the problem", code, stdout, stderr)
	}
}

func TestBigQueryNeedsItsSchemaFiles(t *testing.T) {
	config := warehouse(t, "", map[string]string{})

	code, _, stderr := execute(t, "check", "--config", config)

	if code != 2 || !strings.Contains(stderr, `schemas "schemas" isn't a directory`) {
		t.Errorf("check: got exit code %d and %q, want 2 and a missing schemas directory", code, stderr)
	}

	code, _, stderr = execute(t, "views", "bigquery", "--config", config)

	if code != 2 || !strings.Contains(stderr, "bq show --schema") {
		t.Errorf("views: got exit code %d and %q, want 2 and the bq command that writes the schema file", code, stderr)
	}
}

func TestABigQuerySectionNeedsSchemas(t *testing.T) {
	config := project(t, map[string]string{
		cli.ConfigName:                 "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n[bigquery]\ndataset = \"warehouse\"\nproject = \"acme\"\ntables = \"events\"\nviews = \"masked\"\n",
		"catalog/warehouse/events.pii": "id allow\n",
		"stray.json":                   "[]",
	})

	code, _, stderr := execute(t, "check", "--config", config)

	if code != 2 || !strings.Contains(stderr, "[bigquery] needs a dataset, project, tables, views and schemas") {
		t.Errorf("got exit code %d and %q, want 2 and the missing key", code, stderr)
	}
}

func TestCheckComparesBigQueryWithItsSchemaFiles(t *testing.T) {
	code, stdout, stderr := execute(t, "check", "--config", exampleConfig)

	want := "the catalog is valid\nwarehouse matches the schema files\n"

	if code != 0 || stdout != want {
		t.Errorf("got exit code %d, %q and %q, want 0 and %q", code, stdout, stderr, want)
	}
}

func TestCheckReportsEveryBigQueryProblem(t *testing.T) {
	config := project(t, map[string]string{
		cli.ConfigName:                 "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n" + bigquerySection,
		"catalog/warehouse/events.pii": "id allow\nname deny=person\ngone deny=person\n",
		"schemas/events.json":          `[{"name": "id", "type": "STRING"}, {"name": "name", "type": "STRING"}, {"name": "phone", "type": "STRING"}]`,
	})

	code, _, stderr := execute(t, "check", "--config", config)

	if code != 1 {
		t.Fatalf("exit code %d, want 1: %s", code, stderr)
	}

	for _, want := range []string{"warehouse events.phone: not classified", "warehouse events.gone: not in the schema"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in %q", want, stderr)
		}
	}
}

func TestABigQueryMaskCantBeAnHMAC(t *testing.T) {
	config := project(t, map[string]string{
		cli.ConfigName: "catalog = \".\"\n[policy]\nreasons = [\"person\"]\n[[policy.maskers]]\nterm = \"a\"\nsuffix = \"a\"\nkinds = [\"text\"]\n" +
			bigquerySection + "masks.a = { hmac = { scope = \"s\", key = \"k\" } }\n",
	})

	code, _, stderr := execute(t, "views", "bigquery", "--config", config)

	if code != 2 || !strings.Contains(stderr, "this dialect has no hmac") {
		t.Errorf("got exit code %d and %q, want 2 and no hmac", code, stderr)
	}
}
