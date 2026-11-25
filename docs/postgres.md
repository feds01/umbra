# PostgreSQL

`postgres.Views` renders a view of each table: a view in `Schema` of the table in
`Source` that shows only what each column's class allows, with the class's label as
each column's comment.

```go
views := postgres.Views{
	Schema:  "masked",    // the views
	Source:  "public",    // the tables
	Role:    "analyst",   // granted SELECT on each view
	Masks:   masks,       // an expression for each masker's term
	Replace: true,        // drop each view first, for a migration that changes one
}

sql, err := views.SQL(catalog, "postgres")            // every table
sql, err = views.SQL(catalog, "postgres", "contacts") // or the ones named
```

The output for a contact table:

```sql
{{#include ../cmd/umbra/testdata/views_postgres_replace.sql}}
```

## Masks

A `Mask` is a function from a quoted column to a SQL expression. A column masked by a
term with no mask is hidden.

- `postgres.Extract(pattern)` shows, in lower case, what the pattern's one group
  captures, and `NULL` for a value it doesn't match. `umbra.EmailPattern` captures an
  address's domain, and `umbra.URLPattern` a URL's host. Both match only a whole,
  well-formed value.
- `postgres.HMAC(scope, key)` shows `p_` and 16 hex digits of an HMAC-SHA256 of the
  scope column and the value. It is the same for equal values within a scope, and
  differs across them. `key` is SQL that yields the key, and the `hmac` function is
  pgcrypto's, from the `public` schema.

## The classification view

`views.ClassificationSQL(catalog, "postgres", "classification")` renders a view that
lists every column of every table, the denied ones included: its class, the reason
for it, and the view columns that show it. It lets an analyst see what they can't
read, and why.

Its columns are `table_name`, `column_name`, `class`, `reason` and `visible_as`:

```text
 table_name | column_name | class                           | reason | visible_as
------------+-------------+---------------------------------+--------+-----------------------------
 contacts   | name        | deny=person                     | person |
 contacts   | email       | generalise=domain,pseudonymise  | person | email_domain,email_pseudonym
```

## What it leaves out

- A table in another schema, named `schema.table` in the catalog, has no view here.
- A table with nothing to show gets no view, though `Replace` still drops its old one.
