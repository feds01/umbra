# umbra: guide for contributors and agents

umbra classifies database columns for personal data, and renders the masked
views that hide it. The user guide in [docs](docs/SUMMARY.md) describes its
behaviour: when a change alters what users see, update the guide in the same
change.

## Commands

```sh
just test          # all tests, in every module; extra args go to go test
just lint          # golangci-lint (formatters included) + umbra's own contracts
just fmt           # format
just integration   # the tests against PostgreSQL, in Docker
just update-golden # rewrite the golden files, the command's too; review `git diff`
just run version   # build the command and run it here: `just run check --config cmd/umbra/testdata/umbra.toml`
just book          # serve the user guide in docs (mdBook)
just changelog <version>  # write CHANGELOG.md from the titles of the merged PRs
```

umbra checks its own architecture: the contracts in the root `zenith.toml` run in
`just lint`, in the pre-commit hook and in CI. The library imports only the
standard library, which a contract enforces: a dependency belongs in `cmd/umbra`.

## Architecture

The root package classifies, each target renders for its own dialect, and the command
glues. The targets stand alone: the library's `postgres` and `bigquery` never import
each other or the command, the root imports neither, and the command's two targets are
packages of their own. `zenith.toml` enforces it. The modules and packages, and what
each owns:

- umbra (the root): classes and tags, the policy, the registry of models, the
  catalog and its `.pii` files, `Table.Exposures` (the plan of a view), and
  `Catalog.Check` against a schema.
- `postgres`: the PostgreSQL views, the classification view, and `ReadSchema`,
  the only code that opens a database.
- `bigquery`: the BigQuery views over records, `Flatten`, and `ParseSchema`, which
  reads the schema that `bq show --schema` prints.
- `internal/golden`: golden-file comparison for tests.
- `cmd/umbra` (a module of its own, and the only one with dependencies): `main`, the
  dispatch of commands, and the `umbra.toml` that joins the targets' sections.
  - `internal/postgres` and `internal/bigquery`: a target each, owning its section of
    `umbra.toml`, its views and its schema check. Each uses only its own library
    package, which `architecture_test.go` holds it to, and `internal/cli`.
  - `internal/cli`: what the targets share, the project's catalog and policy, the
    masks, the flags and the exit codes. It knows no target.
- `integration` (a module of its own): tests against a real PostgreSQL.

Key invariants:

- **The library is stdlib-only.** No globals and no `init` registration.
- **Collect errors; don't panic.** Report every problem at once with
  `errors.Join`, each naming its dataset, table and column.
- **Output must be deterministic.** Sort before rendering.
- **The zero value is the safe one.** `Kind`'s zero is `Opaque`, which no masker reads.
- **One grammar.** `ParseTag(s).String() == s`, and a fuzz test holds it.
- **The command's exit codes:** 0 found nothing, 1 the catalog has problems, 2 umbra
  couldn't run. SQL goes to stdout, problems and errors to stderr.

## Conventions

- Every exported name gets a doc comment that is a sentence. Examples are the
  docs: each `Example` checks its output.
- Golden files are rewritten with `-update`, and reviewed in the diff.
- Number the key steps of a command with comments: `// 1. Load the catalog.`
- Keep commands thin: logic that isn't about the command line belongs in the
  library or a target.
- Use British spelling in prose (colour, normalise), but PostgreSQL's
  "materialized".
- Commits are conventional (`feat(cli): …`, `fix(postgres): …`). Branches are
  `feds01/<topic>`.
- **Never commit details of a private codebase.** A real policy appears only as
  "a policy like …" in examples.
