# Concepts

## Classes

Every column has one class:

| Tag | Class | A view shows |
|---|---|---|
| `allow` | `umbra.Allow` | the value as it is |
| `allow=<basis>` | `umbra.Allow` | the value, for a column that can hold anything, such as JSON, with the reason it is safe |
| `deny=<reason>` | `umbra.Deny` | nothing |
| `<term>,<term>` | `umbra.Mask` | a value derived from the column for each term |

`allow` and `deny` need no SQL, so umbra owns them. Every other term is a **masker**
that your policy declares.

A **zero value is the safe one**: a column whose type umbra doesn't recognise is
`Opaque`, which no masker can read, so only an `allow` with a basis shows it.

## The policy

A policy declares what a tag may say:

- **Reasons** to deny a column, such as `person` or `credential`.
- **Bases** to allow a column that can hold anything.
- **Maskers**, each with a *term* (`generalise=domain`), the *suffix* of the view
  column it shows (`email` becomes `email_domain`), the *kinds* of value it can read
  (text, uuid, inet or scalar), and optionally the columns it *reads*: a pseudonym
  keyed by `org_id` needs the table to allow `org_id`.
- **Rules**, your own checks, run on each table. A rule is a Go function, so it
  lives in Go and not in `umbra.toml`.

## Tags and catalog files

One grammar is used everywhere: the struct tag, a line of a catalog file, and the
comment on a view column (`pii:generalise=domain`) all print the same class, and
`ParseTag(s).String() == s`.

```text
tag = "allow" [ "=" basis ] | "deny=" reason | term { "," term }
```

A catalog file is `<dataset>/<table>.pii`, a column and its tag to a line, with no
padding so that a change is a one-line diff. Lines that start with `#` are comments.
A **dataset** is a namespace for tables, such as `postgres` or `warehouse`.

A column of a BigQuery record is named by its dotted path: `properties.email`.

## Registering models

umbra can't list your Go types at runtime, so a registry lists them, one line per
table:

```go
umbra.Model[Contact](registry.Dataset("postgres"), "contacts",
	umbra.Tag("legacy_source", "deny=internal"), // a column no field stores
)

registry.Dataset("postgres").Columns("schema_migrations", // a table with no model
	umbra.Tag("version", "allow"),
)
```

- A field with no tag is an error, so a new column can't be forgotten. `pii:"-"`
  skips a field that isn't a column, such as a relation.
- A **mapper** names each field's column. `umbra.SnakeCase` is the default, and you
  can pass your ORM's own, so the catalog and the ORM never disagree.
- Embedded structs contribute their fields.

## Exposures

`Table.Exposures` is the plan of a view: what it shows, under which names, in which
order. It is the same for every target. A target maps each term to an expression in
its own dialect, and a term it can't compute, such as a keyed hash in BigQuery, is
missing from its map: the column stays hidden.
