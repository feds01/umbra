// Package integration tests umbra against a real PostgreSQL server. It is a module of its
// own, so umbra itself depends on nothing but the standard library.
//
// Set UMBRA_TEST_DATABASE_URL to a server whose user may create databases and roles:
//
//	UMBRA_TEST_DATABASE_URL='postgres://localhost:5432/postgres?sslmode=disable' go test ./...
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/feds01/umbra"
	"github.com/feds01/umbra/postgres"
)

func TestReadSchemaSeesEveryRelationAndResolvesDomains(t *testing.T) {
	db, _ := database(t)

	execute(t, db,
		`CREATE TYPE mood AS ENUM ('calm', 'busy')`,
		`CREATE DOMAIN document AS jsonb`,
		`CREATE DOMAIN nested_document AS document`,
		`CREATE DOMAIN short_text AS character varying(20)`,
		`CREATE TABLE contacts (
			id uuid, email character varying(255), settings jsonb, profile document, history nested_document,
			mood mood, tags text[], nickname short_text, created_at timestamp(3) with time zone
		)`,
		`CREATE SCHEMA other`,
		`CREATE TABLE other.leaked (id uuid)`,
		`CREATE MATERIALIZED VIEW contacts_by_id AS SELECT id FROM contacts`,
		`CREATE VIEW plain AS SELECT id FROM contacts`,
		`CREATE TABLE events (id uuid, day date) PARTITION BY RANGE (day)`,
		`CREATE TABLE events_2026 PARTITION OF events FOR VALUES FROM ('2026-01-01') TO ('2027-01-01')`,
	)

	schema, err := postgres.ReadSchema(t.Context(), db, "public")
	if err != nil {
		t.Fatal(err)
	}

	uuid := umbra.Type{Name: "uuid", Kind: umbra.UUID}
	want := umbra.Schema{
		"contacts": {
			"id":         uuid,
			"email":      {Name: "character varying(255)", Kind: umbra.Text},
			"settings":   {Name: "jsonb"},
			"profile":    {Name: "jsonb"},
			"history":    {Name: "jsonb"},
			"mood":       {Name: "enum", Kind: umbra.Scalar},
			"tags":       {Name: "text[]", Kind: umbra.Text, Array: true},
			"nickname":   {Name: "character varying(20)", Kind: umbra.Text},
			"created_at": {Name: "timestamp(3) with time zone", Kind: umbra.Scalar},
		},
		"other.leaked":   {"id": uuid},
		"contacts_by_id": {"id": uuid},
		"events":         {"id": uuid, "day": {Name: "date", Kind: umbra.Scalar}},
	}

	if !reflect.DeepEqual(schema, want) {
		t.Errorf("got\n%v\nwant\n%v", schema, want)
	}
}

func TestTheViewsShowOnlyWhatTheClassesAllow(t *testing.T) {
	db, role := database(t)

	execute(t, db,
		`CREATE EXTENSION pgcrypto`,
		`CREATE TABLE contacts (id uuid, org_id uuid, name text, email text, website text, last_ip inet)`,
		`CREATE SCHEMA private`,
		`CREATE TABLE private.pseudonym_key (id serial, key bytea)`,
		`INSERT INTO private.pseudonym_key (key) VALUES (gen_random_bytes(32))`,
		`CREATE SCHEMA masked`,
		`GRANT USAGE ON SCHEMA masked TO `+role,
	)

	catalog := app(t)

	schema, err := postgres.ReadSchema(t.Context(), db, "public")
	if err != nil {
		t.Fatal(err)
	}

	if err := catalog.Check("postgres", schema); err != nil {
		t.Fatalf("the catalog doesn't match the schema:\n%v", err)
	}

	views := postgres.Views{
		Schema: "masked",
		Source: "public",
		Role:   role,
		Masks: map[string]postgres.Expr{
			"generalise=domain": postgres.Extract(umbra.EmailPattern),
			"generalise=host":   postgres.Extract(umbra.URLPattern),
			"pseudonymise":      postgres.HMAC("org_id", "SELECT key FROM private.pseudonym_key ORDER BY id DESC LIMIT 1"),
		},
		Replace: true,
	}

	viewsSQL, err := views.SQL(catalog, "postgres")
	if err != nil {
		t.Fatal(err)
	}

	classificationSQL, err := views.ClassificationSQL(catalog, "postgres", "column_classification")
	if err != nil {
		t.Fatal(err)
	}

	execute(t, db,
		viewsSQL,
		classificationSQL,
		`INSERT INTO contacts VALUES
			('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-00000000000a', 'NAME_MARKER',
				'Alice@Example.COM', 'https://Example.org/PATH_MARKER?q=QUERY_MARKER', '10.0.0.1'),
			('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-00000000000a', 'NAME_MARKER',
				'alice@example.com', 'not a url', '10.0.0.1'),
			('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-00000000000b', 'NAME_MARKER',
				'alice@example.com', NULL, '10.0.0.1')`,
	)

	columns, rows := queryAs(t, db, role, `SELECT * FROM masked.contacts ORDER BY id`)

	if want := []string{"id", "org_id", "email_domain", "email_pseudonym", "website_host", "last_ip_pseudonym"}; !slices.Equal(columns, want) {
		t.Fatalf("the view has columns %v, want %v", columns, want)
	}

	for _, row := range rows {
		if line := strings.Join(row, " "); strings.Contains(line, "MARKER") {
			t.Errorf("a hidden value shows: %s", line)
		}
	}

	pseudonym := regexp.MustCompile(`^p_[0-9a-f]{16}$`)
	first, sameOrg, otherOrg := rows[0], rows[1], rows[2]

	switch {
	case first[2] != "example.com" || otherOrg[2] != "example.com":
		t.Errorf("email domains are %q and %q", first[2], otherOrg[2])
	case first[4] != "example.org" || sameOrg[4] != "NULL":
		t.Errorf("website hosts are %q and %q", first[4], sameOrg[4])
	case !pseudonym.MatchString(first[3]) || !pseudonym.MatchString(first[5]):
		t.Errorf("pseudonyms are %q and %q", first[3], first[5])
	case first[3] != sameOrg[3]:
		t.Errorf("an organisation sees one address as %q and %q", first[3], sameOrg[3])
	case first[3] == otherOrg[3]:
		t.Errorf("two organisations see one address as the same pseudonym, %q", first[3])
	}

	_, labels := queryAs(t, db, role, `SELECT col_description('masked.contacts'::regclass, 3)`)
	if labels[0][0] != "pii:generalise=domain" {
		t.Errorf("email_domain is labelled %q", labels[0][0])
	}

	_, classified := queryAs(t, db, role, `SELECT table_name, column_name, class, reason, visible_as
		FROM masked.column_classification WHERE column_name = 'email'`)
	if want := [][]string{{"contacts", "email", "generalise=domain,pseudonymise", "person", "email_domain,email_pseudonym"}}; !reflect.DeepEqual(classified, want) {
		t.Errorf("the classification view says %v", classified)
	}

	transaction, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}

	defer transaction.Rollback()

	execute(t, transaction, `SET LOCAL ROLE `+role)

	if _, err := transaction.ExecContext(t.Context(), `SELECT * FROM public.contacts`); err == nil {
		t.Error("the role reads the table itself")
	}
}

// app is the catalog of the contacts table and the key its pseudonyms use.
func app(t *testing.T) *umbra.Catalog {
	t.Helper()

	registry := umbra.NewRegistry(umbra.Policy{
		Reasons: []umbra.Reason{"credential", "person"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text, Reason: "person"},
			{Term: "generalise=host", Suffix: "host", Kinds: umbra.Text},
			{Term: "pseudonymise", Suffix: "pseudonym", Kinds: umbra.Text | umbra.Inet, Reason: "person", Reads: []string{"org_id"}},
		},
	}, nil)

	public := registry.Dataset("postgres")

	public.Columns("contacts",
		umbra.Tag("id", "allow"),
		umbra.Tag("org_id", "allow"),
		umbra.Tag("name", "deny=person"),
		umbra.Tag("email", "generalise=domain,pseudonymise"),
		umbra.Tag("website", "generalise=host"),
		umbra.Tag("last_ip", "pseudonymise"),
	)
	public.Columns("private.pseudonym_key", umbra.Tag("id", "allow"), umbra.Tag("key", "deny=credential"))

	catalog, err := registry.Catalog()
	if err != nil {
		t.Fatal(err)
	}

	return catalog
}

// database creates a database and a role for one test, and drops both after it.
func database(t *testing.T) (*sql.DB, string) {
	t.Helper()

	address := os.Getenv("UMBRA_TEST_DATABASE_URL")
	if address == "" {
		t.Skip("set UMBRA_TEST_DATABASE_URL to test against PostgreSQL")
	}

	admin, err := sql.Open("postgres", address)
	if err != nil {
		t.Fatal(err)
	}

	name := fmt.Sprintf("umbra_test_%d", time.Now().UnixNano())
	execute(t, admin, `CREATE DATABASE `+name, `CREATE ROLE `+name)

	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}

	parsed.Path = "/" + name

	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()

		for _, statement := range []string{`DROP DATABASE ` + name + ` WITH (FORCE)`, `DROP ROLE ` + name} {
			if _, err := admin.Exec(statement); err != nil {
				t.Error(err)
			}
		}

		admin.Close()
	})

	return db, name
}

// executor runs statements: a database, or a transaction in one.
type executor interface {
	ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error)
}

func execute(t *testing.T, db executor, statements ...string) {
	t.Helper()

	for _, statement := range statements {
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%v:\n%s", err, statement)
		}
	}
}

// queryAs runs a query as the role, and returns its columns and its rows as text.
func queryAs(t *testing.T, db *sql.DB, role, query string) ([]string, [][]string) {
	t.Helper()

	transaction, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}

	defer transaction.Rollback()

	execute(t, transaction, `SET LOCAL ROLE `+role)

	rows, err := transaction.QueryContext(t.Context(), query)
	if err != nil {
		t.Fatal(err)
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	var table [][]string

	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		pointers := make([]any, len(columns))

		for index := range values {
			pointers[index] = &values[index]
		}

		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}

		row := make([]string, len(columns))

		for index, value := range values {
			row[index] = "NULL"
			if value.Valid {
				row[index] = value.String
			}
		}

		table = append(table, row)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return columns, table
}
