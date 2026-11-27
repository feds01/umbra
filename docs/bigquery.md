# BigQuery

`bigquery.Views` renders a view over a table's fields, and needs no Google SDK: you
pass the table's schema as `bigquery.Field` values, and create and authorise the
views yourself.

```go
views := bigquery.Views{
	Project: "acme",
	Dataset: "masked", // holds the views
	Masks:   map[string]bigquery.Expr{"generalise=domain": bigquery.Extract(umbra.EmailPattern)},
}

view, err := views.View(table, bigquery.Source{Dataset: "events", Table: "events"}, fields)
// view.SQL creates or replaces the view, and view.Columns are its columns
```

Each field shows what the table's class for its dotted path allows. A field the table
doesn't classify, or whose type its class doesn't fit, is hidden. A record keeps its
shape with only the fields it may show, and a repeated record is rebuilt element by
element:

```sql
{{#include ../cmd/umbra/testdata/views_bigquery.sql}}
```

## Row structs

`bigquery.Columns` is a mapper for the structs BigQuery's Go client reads and writes, so
a table's classes sit on its row struct. It names a column by its `bigquery` tag, and a
record's leaves by their dotted path, repeated or not:

```go
type Event struct {
	ID          string       `bigquery:"id"                   pii:"allow"`
	Geolocation *Geolocation `bigquery:"geolocation,nullable"` // a record: its fields carry the tags
	Body        Payload      `bigquery:"body,json"            pii:"deny=json"`
}

umbra.Model[Event](registry.Dataset("events").MapWith(bigquery.Columns), "events")
```

A record is a field whose struct names one of its fields with a `bigquery` tag. Any
other struct, such as `time.Time`, is one column, as is a record with the `json` option.

## Descriptions

Each view column's description is the class's label, and the view's description lists
how every field is shown. BigQuery limits a column description to 1,024 characters
and a view's to 16,384, and umbra reports an error rather than truncating.

## What BigQuery lacks

There is no keyed hash, so a pseudonym can't be computed: leave `pseudonymise` out of
`Masks`, and those columns stay hidden.

## Schemas

`bq show --schema --format=prettyjson <project>:<dataset>.<table>` prints a table's
fields. The `umbra` command reads that JSON, one file per table, as
[its BigQuery section](command-line.md#bigquery) describes.
