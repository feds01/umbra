package postgres_test

import (
	"fmt"
	"log"

	"github.com/feds01/umbra"
	"github.com/feds01/umbra/postgres"
)

// contacts is the catalog of one table, whose email shows as a domain and a pseudonym.
func contacts() *umbra.Catalog {
	registry := umbra.NewRegistry(umbra.Policy{
		Reasons: []umbra.Reason{"person"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text, Reason: "person"},
			{Term: "pseudonymise", Suffix: "pseudonym", Kinds: umbra.Text, Reason: "person", Reads: []string{"org_id"}},
		},
	}, nil)

	registry.Dataset("postgres").Columns("contacts",
		umbra.Tag("id", "allow"),
		umbra.Tag("org_id", "allow"),
		umbra.Tag("name", "deny=person"),
		umbra.Tag("email", "generalise=domain,pseudonymise"),
	)

	catalog, err := registry.Catalog()
	if err != nil {
		log.Fatal(err)
	}

	return catalog
}

func ExampleViews_SQL() {
	views := postgres.Views{
		Schema: "masked",
		Source: "public",
		Role:   "analyst",
		Masks: map[string]postgres.Expr{
			"generalise=domain": postgres.Extract(`@(.+)$`),
			"pseudonymise":      postgres.HMAC("org_id", "SELECT key FROM private.pseudonym_key"),
		},
		Replace: true,
	}

	sql, err := views.SQL(contacts(), "postgres")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(sql)

	// Output:
	// DROP VIEW IF EXISTS masked."contacts";
	// CREATE VIEW masked."contacts" AS
	// SELECT
	//     "id",
	//     "org_id",
	//     lower(substring("email" FROM '@(.+)$')) AS "email_domain",
	//     'p_' || left(encode(public.hmac(convert_to("org_id"::text || '|' || lower("email"::text), 'UTF8'), (SELECT key FROM private.pseudonym_key), 'sha256'), 'hex'), 16) AS "email_pseudonym"
	// FROM public."contacts";
	// COMMENT ON COLUMN masked."contacts"."id" IS 'pii:allow';
	// COMMENT ON COLUMN masked."contacts"."org_id" IS 'pii:allow';
	// COMMENT ON COLUMN masked."contacts"."email_domain" IS 'pii:generalise=domain';
	// COMMENT ON COLUMN masked."contacts"."email_pseudonym" IS 'pii:pseudonymise';
	// GRANT SELECT ON masked."contacts" TO analyst;
}

func ExampleViews_ClassificationSQL() {
	views := postgres.Views{
		Schema: "masked",
		Role:   "analyst",
		Masks:  map[string]postgres.Expr{"generalise=domain": postgres.Extract(`@(.+)$`)},
	}

	sql, err := views.ClassificationSQL(contacts(), "postgres", "column_classification")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(sql)

	// Output:
	// CREATE OR REPLACE VIEW masked.column_classification AS
	// SELECT * FROM (VALUES
	//     ('contacts', 'id', 'allow', NULL, 'id'),
	//     ('contacts', 'org_id', 'allow', NULL, 'org_id'),
	//     ('contacts', 'name', 'deny=person', 'person', NULL),
	//     ('contacts', 'email', 'generalise=domain,pseudonymise', 'person', 'email_domain')
	// ) AS classification(table_name, column_name, class, reason, visible_as);
	// GRANT SELECT ON masked.column_classification TO analyst;
}
