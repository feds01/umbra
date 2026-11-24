// Package postgres renders the masked views of a catalog's tables for PostgreSQL, and
// reads the schema a database really has for [umbra.Catalog.Check].
package postgres

import (
	"fmt"
	"slices"
	"strings"

	"github.com/feds01/umbra"
)

// Views renders a masked view of each table: a view in Schema of the table in Source
// that shows only what each column's class allows, with the class's label as each
// column's comment.
type Views struct {
	// Schema holds the views, such as "masked". It is written as given.
	Schema string
	// Source holds the tables, such as "public". It is written as given.
	Source string
	// Role is granted SELECT on each view, unless it is empty. It is written as given.
	Role string
	// Masks computes each masker's term. A column masked by a term it lacks is hidden.
	Masks map[string]Expr
	// Replace drops each view before creating it, for a migration that changes one.
	Replace bool
}

// An Expr computes a masked value from a quoted column, such as "email".
type Expr func(column string) string

// Extract shows, in lower case, what the pattern's group captures, and NULL for a value
// it doesn't match.
func Extract(pattern string) Expr {
	return func(column string) string {
		return "lower(substring(" + column + " FROM " + literal(pattern) + "))"
	}
}

// HMAC shows a pseudonym that is the same for equal values within each value of the
// scope column, and differs across them: "p_" and 16 hex digits of an HMAC-SHA256 of the
// scope and the value in lower case. key is SQL that yields the key, and the hmac
// function is pgcrypto's, from the public schema.
func HMAC(scope, key string) Expr {
	return func(column string) string {
		return fmt.Sprintf(
			"'p_' || left(encode(public.hmac(convert_to(%s::text || '|' || lower(%s::text), 'UTF8'), (%s), 'sha256'), 'hex'), 16)",
			identifier(scope), column, key,
		)
	}
}

// SQL returns the statements that create the views of the named tables, or of every
// table in Source when none is named. A table with nothing to show gets no view, though
// Replace still drops its old one. The catalog names a table in another schema
// schema.table; those have no view here.
func (v Views) SQL(catalog *umbra.Catalog, dataset string, tables ...string) (string, error) {
	if len(tables) == 0 {
		for table := range catalog.Tables(dataset) {
			if !strings.Contains(table.Name, ".") {
				tables = append(tables, table.Name)
			}
		}
	}

	var views []string

	for _, name := range tables {
		table, isFound := catalog.Table(dataset, name)

		switch {
		case !isFound:
			return "", fmt.Errorf("%s %s: not in the catalog", dataset, name)
		case strings.Contains(name, "."):
			return "", fmt.Errorf("%s %s: not in the source schema, %s", dataset, name, v.Source)
		}

		if view := v.view(table); view != "" {
			views = append(views, view)
		}
	}

	return strings.Join(views, "\n"), nil
}

func (v Views) view(table umbra.Table) string {
	view := v.Schema + "." + identifier(table.Name)

	var out strings.Builder

	if v.Replace {
		fmt.Fprintf(&out, "DROP VIEW IF EXISTS %s;\n", view)
	}

	exposures := table.Exposures(v.can)
	if len(exposures) == 0 {
		return out.String()
	}

	selected := make([]string, len(exposures))
	for index, exposure := range exposures {
		selected[index] = "    " + v.selected(exposure)
	}

	fmt.Fprintf(&out, "CREATE VIEW %s AS\nSELECT\n%s\nFROM %s.%s;\n",
		view, strings.Join(selected, ",\n"), v.Source, identifier(table.Name))

	for _, exposure := range exposures {
		fmt.Fprintf(&out, "COMMENT ON COLUMN %s.%s IS %s;\n", view, identifier(exposure.Name), literal(exposure.Label))
	}

	if v.Role != "" {
		fmt.Fprintf(&out, "GRANT SELECT ON %s TO %s;\n", view, v.Role)
	}

	return out.String()
}

// selected is an exposure in a select list: its column, or its masked value named for it.
func (v Views) selected(exposure umbra.Exposure) string {
	column := identifier(exposure.Column)
	if exposure.Term == "" {
		return column
	}

	return v.Masks[exposure.Term](column) + " AS " + identifier(exposure.Name)
}

func (v Views) can(term string) bool {
	_, isMasked := v.Masks[term]

	return isMasked
}

// ClassificationSQL returns the statements that create or replace the view name in
// Schema, which lists every column of the dataset's tables, the denied ones included:
// its class, the reason for it, and the view columns that show it.
func (v Views) ClassificationSQL(catalog *umbra.Catalog, dataset, name string) (string, error) {
	var rows []string

	for table := range catalog.Tables(dataset) {
		shownAs := map[string][]string{}
		for _, exposure := range table.Exposures(v.can) {
			shownAs[exposure.Column] = append(shownAs[exposure.Column], exposure.Name)
		}

		for _, column := range table.Columns {
			rows = append(rows, fmt.Sprintf("    (%s, %s, %s, %s, %s)",
				literal(table.Name),
				literal(column.Name),
				literal(column.Class.String()),
				nullable(reason(column.Class)),
				nullable(strings.Join(shownAs[column.Name], ",")),
			))
		}
	}

	if len(rows) == 0 {
		return "", fmt.Errorf("%s: no tables", dataset)
	}

	view := v.Schema + "." + name

	var out strings.Builder

	fmt.Fprintf(&out, "CREATE OR REPLACE VIEW %s AS\nSELECT * FROM (VALUES\n%s\n) AS classification(table_name, column_name, class, reason, visible_as);\n",
		view, strings.Join(rows, ",\n"))

	if v.Role != "" {
		fmt.Fprintf(&out, "GRANT SELECT ON %s TO %s;\n", view, v.Role)
	}

	return out.String(), nil
}

// reason is why a column isn't simply shown: the reason it is denied, its maskers'
// reasons, or the basis that allows it.
func reason(class umbra.Class) string {
	switch class := class.(type) {
	case umbra.Allow:
		return string(class.Basis)
	case umbra.Deny:
		return string(class.Reason)
	case umbra.Mask:
		var reasons []string

		for _, masker := range class.Maskers {
			if masker.Reason != "" && !slices.Contains(reasons, string(masker.Reason)) {
				reasons = append(reasons, string(masker.Reason))
			}
		}

		return strings.Join(reasons, ",")
	}

	return ""
}

// identifier quotes a name, such as a table's.
func identifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// literal quotes a string.
func literal(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// nullable quotes a string, or writes NULL for an empty one.
func nullable(value string) string {
	if value == "" {
		return "NULL"
	}

	return literal(value)
}
