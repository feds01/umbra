# Checking a database

`Catalog.Check` compares a dataset's tables in the catalog with the schema the
database really has. It reports every problem at once:

- `not classified`: a table or column the database has and the catalog doesn't;
- `not in the schema`: one the catalog has and the database no longer does;
- `class does not fit the type`: for example, `generalise=domain` on an array.

```text
postgres contacts.email: generalise=domain reads a single text value, not text[]
postgres contacts.notes: not in the schema
postgres contacts.settings: not classified
postgres users: not classified
```

Each wraps `umbra.ErrUnclassified`, `ErrStale` or `ErrType`, for `errors.Is`.

## Reading the schema

`postgres.ReadSchema(ctx, db, "public")` reads the columns of every table,
partitioned table, materialized view and foreign table outside the system schemas
and extensions. A table in the given schema is named bare, and one in another schema
`schema.table`. A domain takes its base type, and every enum is a scalar.

For BigQuery, pass each table's fields to `bigquery.Flatten`, which lists the leaves
of records by dotted path.

## In a test

```go
func TestCatalogMatchesTheDatabase(t *testing.T) {
	schema, err := postgres.ReadSchema(t.Context(), db, "public")
	if err != nil {
		t.Fatal(err)
	}

	if err := catalog.Check("postgres", schema); err != nil {
		t.Error(err)
	}
}
```

Alongside it, `catalog.Verify(os.DirFS("catalog"))` fails when the committed files
differ from what the registry would write.

## In CI, without Go

```yaml
- name: Check the catalog
  run: umbra check --postgres "$DATABASE_URL"
```

Install the command from the [releases](https://github.com/feds01/umbra/releases)
or with Homebrew, as in [Installation](installation.md). See the
[command line](command-line.md) for what `check` reads and its exit codes.
