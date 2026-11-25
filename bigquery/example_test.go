package bigquery_test

import (
	"fmt"
	"log"

	"github.com/feds01/umbra"
	"github.com/feds01/umbra/bigquery"
)

func ExampleViews_View() {
	registry := umbra.NewRegistry(umbra.Policy{
		Reasons: []umbra.Reason{"person"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text},
			{Term: "pseudonymise", Suffix: "pseudonym", Kinds: umbra.Text, Reads: []string{"org_id"}},
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

	// A copy of the Postgres table in BigQuery takes the Postgres classes. BigQuery has
	// no keyed hash, so the views can't pseudonymise.
	contacts, _ := catalog.Table("postgres", "contacts")

	views := bigquery.Views{
		Project: "acme",
		Dataset: "masked",
		Masks:   map[string]bigquery.Expr{"generalise=domain": bigquery.Extract(`@(.+)$`)},
	}

	view, err := views.View(contacts, bigquery.Source{Dataset: "mirror", Table: "pg_contacts"}, []bigquery.Field{
		{Name: "id", Type: "STRING"},
		{Name: "org_id", Type: "STRING"},
		{Name: "name", Type: "STRING"},
		{Name: "email", Type: "STRING"},
		{Name: "_synced_at", Type: "TIMESTAMP"}, // added by the copy, and not classified
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(view.Columns)
	fmt.Println(view.SQL)

	// Output:
	// [id org_id email_domain]
	// CREATE OR REPLACE VIEW `acme.masked.pg_contacts` (
	//   `id` OPTIONS(description="pii:allow"),
	//   `org_id` OPTIONS(description="pii:allow"),
	//   `email_domain` OPTIONS(description="pii:generalise=domain")
	// )
	// OPTIONS(description="Masked view of pg_contacts. A field not listed as exposed is not available. Classification: id: allow, exposed; org_id: allow, exposed; name: deny=person, not exposed; email: generalise=domain,pseudonymise, exposed as email_domain; _synced_at: not classified, not exposed.")
	// AS SELECT
	//   `id`,
	//   `org_id`,
	//   LOWER(REGEXP_EXTRACT(`email`, r'@(.+)$')) AS `email_domain`
	// FROM `acme.mirror.pg_contacts`
}

func ExampleFlatten() {
	registry := umbra.NewRegistry(umbra.Policy{
		Reasons: []umbra.Reason{"person"},
		Maskers: []umbra.Masker{{Term: "generalise=host", Suffix: "host", Kinds: umbra.Text}},
	}, nil)

	registry.Dataset("bigquery").Columns("page_views",
		umbra.Tag("id", "allow"),
		umbra.Tag("page.url", "generalise=host"),
		umbra.Tag("page.title", "allow"),
	)

	catalog, err := registry.Catalog()
	if err != nil {
		log.Fatal(err)
	}

	fields := []bigquery.Field{
		{Name: "id", Type: "STRING"},
		{Name: "page", Type: "RECORD", Fields: []bigquery.Field{
			{Name: "url", Type: "STRING"},
			{Name: "referrer", Type: "STRING"},
		}},
	}

	fmt.Println(catalog.Check("bigquery", umbra.Schema{"page_views": bigquery.Flatten(fields)}))

	// Output:
	// bigquery page_views.page.referrer: not classified
	// bigquery page_views.page.title: not in the schema
}

func ExampleParseSchema() {
	// The output of: bq show --schema --format=prettyjson acme:events.page_views
	fields, err := bigquery.ParseSchema([]byte(`[
		{"name": "id", "type": "STRING", "mode": "NULLABLE"},
		{"name": "tags", "type": "STRING", "mode": "REPEATED"},
		{"name": "page", "type": "RECORD", "mode": "NULLABLE", "fields": [
			{"name": "url", "type": "STRING", "mode": "NULLABLE"}
		]}
	]`))
	if err != nil {
		log.Fatal(err)
	}

	for _, field := range fields {
		fmt.Println(field.Name, field.Type, field.Repeated, len(field.Fields))
	}

	// Output:
	// id STRING false 0
	// tags STRING true 0
	// page RECORD false 1
}
