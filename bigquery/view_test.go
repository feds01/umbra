package bigquery_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/feds01/umbra"
	"github.com/feds01/umbra/bigquery"
	"github.com/feds01/umbra/internal/golden"
)

func policy() umbra.Policy {
	return umbra.Policy{
		Reasons: []umbra.Reason{"credential", "person", "internal"},
		Bases:   []umbra.Basis{"business_data"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text, Reason: "person"},
			{Term: "generalise=host", Suffix: "host", Kinds: umbra.Text},
			{Term: "pseudonymise", Suffix: "pseudonym", Kinds: umbra.Text, Reason: "person"},
		},
	}
}

// table classifies the columns of one table, a column and its tag at a time.
func table(t *testing.T, tags ...string) umbra.Table {
	t.Helper()

	var columns []umbra.Tagged
	for index := 0; index < len(tags); index += 2 {
		columns = append(columns, umbra.Tag(tags[index], tags[index+1]))
	}

	registry := umbra.NewRegistry(policy(), nil)
	registry.Dataset("bigquery").Columns("events", columns...)

	catalog, err := registry.Catalog()
	if err != nil {
		t.Fatal(err)
	}

	events, _ := catalog.Table("bigquery", "events")

	return events
}

// views can generalise, but has no keyed hash to pseudonymise with.
func views() bigquery.Views {
	return bigquery.Views{
		Project: "project",
		Dataset: "masked",
		Masks: map[string]bigquery.Expr{
			"generalise=domain": bigquery.Extract(umbra.EmailPattern),
			"generalise=host":   bigquery.Extract(umbra.URLPattern),
		},
	}
}

var source = bigquery.Source{Dataset: "app", Table: "events", Origin: "the events table"}

func text(name string) bigquery.Field {
	return bigquery.Field{Name: name, Type: "STRING"}
}

func record(name string, fields ...bigquery.Field) bigquery.Field {
	return bigquery.Field{Name: name, Type: "RECORD", Fields: fields}
}

func TestAViewKeepsRecordsTheirShapeAndRenamesWhatItMasks(t *testing.T) {
	places := record("places", text("code"), text("note"), text("site"), record("city", text("label")))
	places.Repeated = true

	fields := []bigquery.Field{
		text("id"),
		text("secret"),
		text("contact"),
		record("properties", text("url"), text("email"), text("ip_addr")),
		record("hidden", text("name")),
		places,
	}

	events := table(t,
		"id", "allow",
		"secret", "deny=credential",
		"contact", "generalise=domain,pseudonymise",
		"properties.url", "generalise=host",
		"properties.email", "generalise=domain",
		"properties.ip_addr", "deny=person",
		"hidden.name", "deny=person",
		"places.code", "allow",
		"places.note", "deny=person",
		"places.site", "generalise=host",
		"places.city.label", "allow",
	)

	view, err := views().View(events, source, fields)
	if err != nil {
		t.Fatal(err)
	}

	golden.Check(t, "events.sql", view.SQL)

	if want := []string{"id", "contact_domain", "properties", "places"}; strings.Join(view.Columns, ",") != strings.Join(want, ",") {
		t.Errorf("columns are %v, want %v", view.Columns, want)
	}
}

func TestAViewShowsNothingItWasNotToldTo(t *testing.T) {
	fields := []bigquery.Field{
		text("id"),
		text("unclassified"),
		text("visitor"),
		{Name: "count", Type: "INTEGER"},
		{Name: "document", Type: "JSON"},
		{Name: "tags", Type: "STRING", Repeated: true},
	}

	events := table(t,
		"id", "allow",
		"visitor", "pseudonymise",
		"count", "generalise=domain",
		"document", "allow",
		"tags", "generalise=host",
	)

	view, err := views().View(events, source, fields)
	if err != nil {
		t.Fatal(err)
	}

	if len(view.Columns) != 1 || view.Columns[0] != "id" {
		t.Errorf("columns are %v", view.Columns)
	}

	for _, note := range []string{
		"id: allow, exposed",
		"unclassified: not classified, not exposed",
		"visitor: pseudonymise, not exposed",
		"count: generalise=domain, not exposed",
		"document: allow, not exposed",
		"tags: generalise=host, not exposed",
	} {
		if !strings.Contains(view.SQL, note) {
			t.Errorf("the description doesn't say %q", note)
		}
	}
}

func TestAViewWithNothingToShowIsDropped(t *testing.T) {
	fields := []bigquery.Field{text("secret"), record("properties", text("ip_addr"))}

	view, err := views().View(table(t, "secret", "deny=credential", "properties.ip_addr", "deny=person"), source, fields)
	if err != nil {
		t.Fatal(err)
	}

	if want := "DROP VIEW IF EXISTS `project.masked.events`"; view.SQL != want || len(view.Columns) != 0 {
		t.Errorf("got %q, %v", view.SQL, view.Columns)
	}
}

func TestAViewRefusesWhatBigQueryWould(t *testing.T) {
	var (
		many   []bigquery.Field
		manyTo []string
	)

	for index := range 200 {
		name := fmt.Sprintf("field_%0100d", index)
		many = append(many, text(name))
		manyTo = append(manyTo, name, "allow")
	}

	tests := map[string]struct {
		events umbra.Table
		fields []bigquery.Field
		want   string
	}{
		"two fields shown under one name": {
			table(t, "SITE_HOST", "allow", "site", "generalise=host"),
			[]bigquery.Field{text("SITE_HOST"), text("site")},
			"events: two exposed fields are both named site_host",
		},
		"two fields of a record shown under one name": {
			table(t, "places.SITE_HOST", "allow", "places.site", "generalise=host"),
			[]bigquery.Field{record("places", text("SITE_HOST"), text("site"))},
			"events: two exposed fields are both named places.site_host",
		},
		"a description over BigQuery's limit": {
			table(t, manyTo...),
			many,
			"events: the description is 24896 characters, over BigQuery's limit of 16384",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := views().View(test.events, source, test.fields)
			if err == nil || err.Error() != test.want {
				t.Errorf("got %v, want %s", err, test.want)
			}
		})
	}
}

func TestExtractQuotesAPatternARawStringCantHold(t *testing.T) {
	tests := map[string]string{
		`@(.+)$`:   "LOWER(REGEXP_EXTRACT(`email`, r'@(.+)$'))",
		`it's(.+)`: "LOWER(REGEXP_EXTRACT(`email`, \"it's(.+)\"))",
		`(\w+)\`:   "LOWER(REGEXP_EXTRACT(`email`, \"(\\\\w+)\\\\\"))",
		"a\n(b)":   "LOWER(REGEXP_EXTRACT(`email`, \"a\\n(b)\"))",
	}

	for pattern, want := range tests {
		if got := bigquery.Extract(pattern)("`email`"); got != want {
			t.Errorf("%q: got %s, want %s", pattern, got, want)
		}
	}
}

func TestFlattenNamesEachLeafByItsPath(t *testing.T) {
	places := record("places", text("code"), bigquery.Field{Name: "visits", Type: "INT64", Repeated: true})
	places.Repeated = true

	leaves := bigquery.Flatten([]bigquery.Field{
		text("id"),
		{Name: "payload", Type: "JSON"},
		record("properties", text("email"), record("geo", bigquery.Field{Name: "lat", Type: "FLOAT64"})),
		places,
	})

	want := map[string]umbra.Type{
		"id":                 {Name: "STRING", Kind: umbra.Text},
		"payload":            {Name: "JSON"},
		"properties.email":   {Name: "STRING", Kind: umbra.Text},
		"properties.geo.lat": {Name: "FLOAT64", Kind: umbra.Scalar},
		"places.code":        {Name: "STRING", Kind: umbra.Text},
		"places.visits":      {Name: "ARRAY<INT64>", Kind: umbra.Scalar, Array: true},
	}

	if fmt.Sprint(leaves) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", leaves, want)
	}
}
