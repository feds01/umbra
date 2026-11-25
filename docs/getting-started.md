# Getting started

The README shows a tagged struct becoming a view in a dozen lines. This page puts
the same pieces in a project, where they have to stay true as the schema changes.
Without Go, the [command line](command-line.md) does the same from files.

## Layout

One package owns everything that classifies:

```text
internal/pii/
  policy.go        the vocabulary of tags
  registry.go      every model, one line per table
  catalog/         the generated .pii files, committed
  gen/main.go      writes catalog/
  pii_test.go      keeps the files and the database honest
```

## The policy and the registry

`policy.go` is a function, so a test and the generator build the same one:

```go
func Policy() umbra.Policy {
	return umbra.Policy{
		Reasons: []umbra.Reason{"person", "credential", "internal"},
		Maskers: []umbra.Masker{
			{Term: "generalise=domain", Suffix: "domain", Kinds: umbra.Text, Reason: "person"},
		},
	}
}
```

`registry.go` lists the models. This is the one place a new table has to be added,
and forgetting it fails the check below:

```go
func Registry() *umbra.Registry {
	registry := umbra.NewRegistry(Policy(), umbra.SnakeCase)

	postgres := registry.Dataset("postgres")
	umbra.Model[Contact](postgres, "contacts")
	umbra.Model[Account](postgres, "accounts")
	postgres.Columns("schema_migrations", // the migration tool's table has no model
		umbra.Tag("version", "allow"),
	)

	return registry
}
```

## Generating the files

```go
//go:generate go run ./gen

func main() {
	catalog, err := pii.Registry().Catalog()
	if err != nil {
		log.Fatal(err) // every bad tag at once
	}

	if err := catalog.WriteDir("catalog"); err != nil {
		log.Fatal(err)
	}
}
```

`WriteDir` rewrites only the files that changed, and removes those of tables that no
longer exist, so a pull request's diff is the columns that changed.

## Keeping it honest

Two tests hold the project together. The first fails when the committed files differ
from what the registry would write, so nobody edits one by hand or forgets
`go generate`:

```go
func TestCatalogIsGenerated(t *testing.T) {
	catalog, err := Registry().Catalog()
	if err != nil {
		t.Fatal(err)
	}

	if err := catalog.Verify(os.DirFS("catalog")); err != nil {
		t.Error(err)
	}
}
```

The second compares the catalog with a real database, which is what turns a new
column into a failing build. See [Checking a database](checking.md).

## Using the catalog

Embed the files, and load them where the program needs them, without importing a
single model:

```go
//go:embed catalog
var files embed.FS

func Catalog() (*umbra.Catalog, error) {
	catalog, err := fs.Sub(files, "catalog")
	if err != nil {
		return nil, err
	}

	return umbra.Load(catalog, Policy())
}
```

The same catalog renders the views for a migration, as in [PostgreSQL](postgres.md)
and [BigQuery](bigquery.md).
