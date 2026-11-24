package postgres_test

import (
	"testing"

	"github.com/feds01/umbra"
	"github.com/feds01/umbra/internal/golden"
	"github.com/feds01/umbra/postgres"
)

// app is the catalog of a SaaS app's tables: some to allow, some to mask, a table with
// nothing to show, and a table in another schema.
func app(t *testing.T) *umbra.Catalog {
	t.Helper()

	registry := umbra.NewRegistry(umbra.Policy{
		Reasons: []umbra.Reason{"credential", "person", "free_text"},
		Bases:   []umbra.Basis{"business_data"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text, Reason: "person"},
			{Term: "generalise=host", Suffix: "host", Kinds: umbra.Text},
			{Term: "pseudonymise", Suffix: "pseudonym", Kinds: umbra.Text | umbra.Inet, Reason: "person", Reads: []string{"org_id"}},
		},
	}, nil)

	public := registry.Dataset("postgres")

	public.Columns("accounts",
		umbra.Tag("id", "allow"),
		umbra.Tag("org_id", "allow"),
		umbra.Tag("website", "generalise=host"),
		umbra.Tag("profile", "allow=business_data"),
		umbra.Tag("notes", "deny=free_text"),
	)
	public.Columns("contacts",
		umbra.Tag("id", "allow"),
		umbra.Tag("org_id", "allow"),
		umbra.Tag("name", "deny=person"),
		umbra.Tag("email", "generalise=domain,pseudonymise"),
		umbra.Tag("last_ip", "pseudonymise"),
	)
	public.Columns("api_keys", umbra.Tag("secret", "deny=credential"))
	public.Columns("private.pseudonym_key", umbra.Tag("id", "allow"), umbra.Tag("key", "deny=credential"))

	catalog, err := registry.Catalog()
	if err != nil {
		t.Fatal(err)
	}

	return catalog
}

func views() postgres.Views {
	return postgres.Views{
		Schema: "masked",
		Source: "public",
		Role:   "analyst",
		Masks: map[string]postgres.Expr{
			"generalise=domain": postgres.Extract(umbra.EmailPattern),
			"generalise=host":   postgres.Extract(umbra.URLPattern),
			"pseudonymise":      postgres.HMAC("org_id", "SELECT key FROM private.pseudonym_key ORDER BY id DESC LIMIT 1"),
		},
		Replace: true,
	}
}

func TestViewsMatchTheGoldenFiles(t *testing.T) {
	catalog := app(t)

	sql, err := views().SQL(catalog, "postgres")
	if err != nil {
		t.Fatal(err)
	}

	golden.Check(t, "views.sql", sql)

	classification, err := views().ClassificationSQL(catalog, "postgres", "column_classification")
	if err != nil {
		t.Fatal(err)
	}

	golden.Check(t, "classification.sql", classification)
}

func TestViewsSQL(t *testing.T) {
	withoutReplace := views()
	withoutReplace.Replace = false

	withoutPseudonyms := views()
	withoutPseudonyms.Replace = false
	delete(withoutPseudonyms.Masks, "pseudonymise")

	withoutRole := views()
	withoutRole.Role = ""

	tests := map[string]struct {
		views  postgres.Views
		tables []string
		want   string
	}{
		"a table with nothing to show": {
			withoutReplace, []string{"api_keys"}, "",
		},
		"a table with nothing to show, replaced": {
			views(), []string{"api_keys"}, "DROP VIEW IF EXISTS masked.\"api_keys\";\n",
		},
		"a term the views can't compute": {
			withoutPseudonyms, []string{"contacts"}, "CREATE VIEW masked.\"contacts\" AS\n" +
				"SELECT\n" +
				"    \"id\",\n" +
				"    \"org_id\",\n" +
				"    lower(substring(\"email\" FROM '" + umbra.EmailPattern + "')) AS \"email_domain\"\n" +
				"FROM public.\"contacts\";\n" +
				"COMMENT ON COLUMN masked.\"contacts\".\"id\" IS 'pii:allow';\n" +
				"COMMENT ON COLUMN masked.\"contacts\".\"org_id\" IS 'pii:allow';\n" +
				"COMMENT ON COLUMN masked.\"contacts\".\"email_domain\" IS 'pii:generalise=domain';\n" +
				"GRANT SELECT ON masked.\"contacts\" TO analyst;\n",
		},
		"no role": {
			withoutRole, []string{"api_keys", "accounts"}, "DROP VIEW IF EXISTS masked.\"api_keys\";\n" +
				"\n" +
				"DROP VIEW IF EXISTS masked.\"accounts\";\n" +
				"CREATE VIEW masked.\"accounts\" AS\n" +
				"SELECT\n" +
				"    \"id\",\n" +
				"    \"org_id\",\n" +
				"    lower(substring(\"website\" FROM '" + umbra.URLPattern + "')) AS \"website_host\",\n" +
				"    \"profile\"\n" +
				"FROM public.\"accounts\";\n" +
				"COMMENT ON COLUMN masked.\"accounts\".\"id\" IS 'pii:allow';\n" +
				"COMMENT ON COLUMN masked.\"accounts\".\"org_id\" IS 'pii:allow';\n" +
				"COMMENT ON COLUMN masked.\"accounts\".\"website_host\" IS 'pii:generalise=host';\n" +
				"COMMENT ON COLUMN masked.\"accounts\".\"profile\" IS 'pii:allow=business_data';\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sql, err := test.views.SQL(app(t), "postgres", test.tables...)
			if err != nil {
				t.Fatal(err)
			}

			if sql != test.want {
				t.Errorf("got\n%s\nwant\n%s", sql, test.want)
			}
		})
	}
}

func TestViewsRefuseATableTheyCantRender(t *testing.T) {
	tests := map[string]string{
		"leads":                 "postgres leads: not in the catalog",
		"private.pseudonym_key": "postgres private.pseudonym_key: not in the source schema, public",
	}

	for table, want := range tests {
		_, err := views().SQL(app(t), "postgres", table)
		if err == nil || err.Error() != want {
			t.Errorf("%s: got %v, want %s", table, err, want)
		}
	}

	_, err := views().ClassificationSQL(app(t), "mysql", "column_classification")
	if want := "mysql: no tables"; err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}
