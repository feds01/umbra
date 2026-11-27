# Changelog

What changed for umbra's users in each release. The release workflow uses a
release's section as its notes on GitHub.

## [0.1.0] - 2026-11-27

### Features

- **library:** classify columns with `pii` struct tags, keep the catalog in `.pii` files, check it against a database's schema, and render masked PostgreSQL and BigQuery views
- **cli:** add the umbra command, with version ([#8](https://github.com/feds01/umbra/pull/8))
- **cli:** render masked PostgreSQL views from umbra.toml ([#15](https://github.com/feds01/umbra/pull/15))
- **cli:** check a catalog against a database's schema ([#9](https://github.com/feds01/umbra/pull/9))
- **cli:** render masked BigQuery views, and check the schema files ([#10](https://github.com/feds01/umbra/pull/10))
- **bigquery:** read a table's schema from the JSON that bq prints ([#18](https://github.com/feds01/umbra/pull/18))

### Fixes

- **cli:** review fixes to check, BigQuery schemas and the release gate ([#17](https://github.com/feds01/umbra/pull/17))

### Documentation

- a logo, and a lighter README ([#7](https://github.com/feds01/umbra/pull/7))
- add a user guide, published to GitHub Pages ([#13](https://github.com/feds01/umbra/pull/13))
- add CONTRIBUTING, INSTALL and AGENTS.md ([#14](https://github.com/feds01/umbra/pull/14))

