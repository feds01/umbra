# Design

- **Two built-in classes; everything else is a masker.** `allow` and `deny=<reason>` need
  no SQL, so umbra owns them. A policy declares the rest: a term, an output suffix and
  the kinds of value it reads. Adding `truncate=month` changes the policy, not umbra.
- **A class is a sealed interface.** `Allow`, `Deny` and `Mask` sit behind an unexported
  method, so a value like "deny, shown as a domain" can't be built, and a type switch
  handles each. `String` prints the tag back, and a fuzz test holds the round trip.
- **One grammar everywhere.** The struct tag, the catalog line, the view column's comment
  (`pii:generalise=domain`) and the classification view all print `Class.String`. The
  catalog files need no YAML: umbra parses them with the tag parser.
- **The plan is in the root, the syntax in the target.** `Table.Exposures` decides what a
  view shows, its names and its order. A target maps each term to an expression in its
  dialect. A term it can't compute, such as a keyed hash in BigQuery, is a missing map
  entry, and the column stays hidden.
- **The zero value is the safe one.** `Kind` is a bitset whose zero is `Opaque`. A schema
  reader that meets a type it doesn't know leaves it at zero, which no masker reads and
  `allow` passes only with a basis.
- **Reflection, not source parsing.** Go can't list types at runtime, so a registry lists
  them, one line per table. A `Mapper` names each model's columns: `SnakeCase` by
  default, or your ORM's own parser, so the catalog and the ORM never disagree. Each
  dataset may have its own, as one registry can hold a database and a warehouse.
- **Collect errors; don't panic.** `Model` never fails. `Catalog`, `Load` and `Check`
  return every problem in one `errors.Join`, each naming its table, column and field.
- **Keep the generated files.** Runtime code reads the catalog without importing every
  model package, and reviewers read one small file per table. A column is one line with
  no padding, so a change is a one-line diff.
- **A small surface.** No globals and no `init` registration: the list is the point.
  One generic function (`Model[T]`), one iterator (`Catalog.Tables`) and no `Target`
  interface, since nothing would call through it.
