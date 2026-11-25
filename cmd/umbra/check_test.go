package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCheckTheCatalog(t *testing.T) {
	config := project(t, map[string]string{
		configName:                      "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n",
		"catalog/postgres/contacts.pii": "id allow\nname deny=person\n",
	})

	code, stdout, stderr := execute(t, "check", "--config", config)

	if code != 0 || stdout != "the catalog is valid\n" {
		t.Errorf("got exit code %d, %q and %q, want 0 and the catalog is valid", code, stdout, stderr)
	}
}

func TestCheckReportsEveryBadCatalogFile(t *testing.T) {
	config := project(t, map[string]string{
		configName:                      "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n",
		"catalog/postgres/contacts.pii": "id allow\nname deny=nobody\nemail\n",
	})

	code, stdout, stderr := execute(t, "check", "--config", config)

	if code != 1 || stdout != "" {
		t.Fatalf("got exit code %d and %q, want 1 and nothing: %s", code, stdout, stderr)
	}

	for _, want := range []string{"contacts.pii:2", "contacts.pii:3: want a column and its tag"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in %q", want, stderr)
		}
	}
}

func TestCheckTakesOnlyFlags(t *testing.T) {
	for _, argument := range []string{"postgres://localhost/app", "extra"} {
		code, stdout, stderr := execute(t, "check", "--config", exampleConfig, argument)

		if code != 2 || stdout != "" || !strings.Contains(stderr, "takes only flags") {
			t.Errorf("%q: got exit code %d, %q and %q, want 2, nothing and the usage error", argument, code, stdout, stderr)
		}
	}
}

func TestCheckNeedsAPostgresSectionForADatabase(t *testing.T) {
	config := project(t, map[string]string{
		configName:                      "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n",
		"catalog/postgres/contacts.pii": "id allow\n",
	})

	code, _, stderr := execute(t, "check", "--config", config, "--postgres", "postgres://localhost/app")

	if code != 2 || !strings.Contains(stderr, "has no [postgres] section") {
		t.Errorf("got exit code %d and %q, want 2 and the missing section", code, stderr)
	}
}

func TestAnErrorNeverShowsTheDatabasePassword(t *testing.T) {
	for _, address := range []string{
		"postgres://umbra:s3cretpw@localhost:badport/app",
		"host=localhost port=badport user=umbra password=s3cretpw dbname=app",
		"host=localhost port=badport user=umbra password='s3cret pw' dbname=app",
	} {
		code, _, stderr := execute(t, "check", "--config", exampleConfig, "--postgres", address)

		if code != 2 {
			t.Errorf("%q: got exit code %d, want 2: %s", address, code, stderr)
		}

		if strings.Contains(stderr, "s3cret") {
			t.Errorf("%q: the error shows the password: %s", address, stderr)
		}
	}
}

func TestCheckPostgres(t *testing.T) {
	address := os.Getenv("UMBRA_TEST_DATABASE_URL")
	if address == "" {
		t.Skip("set UMBRA_TEST_DATABASE_URL to test against PostgreSQL")
	}

	admin, err := sql.Open("postgres", address)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { admin.Close() })

	name := fmt.Sprintf("umbra_cli_test_%d", time.Now().UnixNano())

	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE " + name + " WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})

	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}

	parsed.Path = "/" + name

	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { db.Close() })

	for _, statement := range []string{
		`CREATE TABLE contacts (id uuid, org_id uuid, name text, email text, website text, last_ip inet)`,
		`CREATE TABLE api_keys (id uuid, secret text)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, stderr := execute(t, "check", "--config", exampleConfig, "--postgres", parsed.String())

	if code != 0 {
		t.Fatalf("exit code %d: %s", code, stderr)
	}

	if !strings.Contains(stdout, "postgres matches the database") {
		t.Errorf("got %q", stdout)
	}

	if _, err := db.Exec(`ALTER TABLE contacts ADD COLUMN phone text`); err != nil {
		t.Fatal(err)
	}

	code, _, stderr = execute(t, "check", "--config", exampleConfig, "--postgres", parsed.String())

	if code != 1 || !strings.Contains(stderr, "postgres contacts.phone: not classified") {
		t.Errorf("got exit code %d and %q, want 1 and the unclassified phone", code, stderr)
	}

	code, _, stderr = execute(t, "check", "--config", exampleConfig, "--postgres", "postgres://127.0.0.1:1/none?sslmode=disable&connect_timeout=1")

	if code != 2 {
		t.Errorf("got exit code %d and %q, want 2 for a database that can't be reached", code, stderr)
	}
}
