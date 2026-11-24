// Package umbra classifies database columns for personal data, and plans the masked views
// that hide it.
//
// A column's class is a tag on the struct field that stores it:
//
//	type Contact struct {
//		ID    string `pii:"allow"`
//		Name  string `pii:"deny=person"`
//		Email string `pii:"generalise=domain,pseudonymise"`
//	}
//
// allow shows a value as it is, and deny hides it. Every other term names a [Masker] that
// the [Policy] declares, which shows a value derived from the column in its place: here
// email_domain and email_pseudonym.
//
// A [Registry] lists the models, one line per table, and builds a [Catalog]. The catalog
// is kept as one .pii file per table, which runtime code loads without importing a model,
// and [Catalog.Check] compares it with the schema a database really has. The postgres and
// bigquery packages render the masked views.
package umbra
