// Package bigquery renders BigQuery views that mask a table's records as a catalog
// classifies them, for a caller to create and authorize. It needs no Google SDK: a caller
// passes a table's schema as [Field] values.
package bigquery

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/feds01/umbra"
)

// BigQuery's limits on the length of a description.
const (
	maxColumnDescription = 1024
	maxViewDescription   = 16384
)

// Views renders masked views into one dataset.
type Views struct {
	// Project holds the views and the tables they read.
	Project string
	// Dataset holds the views.
	Dataset string
	// Masks computes each masker's term. A field masked by a term it lacks is hidden.
	Masks map[string]Expr
}

// An Expr computes a masked value from a field's reference, such as `properties`.`email`.
type Expr func(field string) string

// Extract shows, in lower case, what the pattern's group captures, and NULL for a value
// it doesn't match.
func Extract(pattern string) Expr {
	return func(field string) string {
		return "LOWER(REGEXP_EXTRACT(" + field + ", " + regexLiteral(pattern) + "))"
	}
}

// A Source is the table a view reads.
type Source struct {
	Dataset string
	Table   string
	// Origin says in words which table the view masks, for its description. It defaults
	// to the table's name.
	Origin string
}

// A Field is a column of a BigQuery schema, or a field of a record.
type Field struct {
	Name string
	// Type is BigQuery's name for it, such as STRING, or RECORD for a record of Fields.
	Type     string
	Repeated bool
	Fields   []Field
}

// A View is the masked view of one table.
type View struct {
	// SQL creates or replaces the view, or drops it when nothing may be shown.
	SQL string
	// Columns are the view's columns, and empty when SQL drops it.
	Columns []string
}

// View returns the masked view of source, named after it. Each field shows what the
// table's class for its dotted path allows, and a field the table doesn't classify, or
// whose type its class doesn't fit, is hidden. A record keeps its shape with only the
// fields it may show. The view's description says how each field is shown.
func (v Views) View(table umbra.Table, source Source, fields []Field) (View, error) {
	name := quote(v.Project + "." + v.Dataset + "." + source.Table)

	plan := newPlan(table, Flatten(fields), v.Masks)

	columns, _, err := plan.fields("", "", fields, 0)
	if err != nil {
		return View{}, fmt.Errorf("%s: %w", source.Table, err)
	}

	if len(columns) == 0 {
		return View{SQL: "DROP VIEW IF EXISTS " + name}, nil
	}

	origin := source.Origin
	if origin == "" {
		origin = source.Table
	}

	description := fmt.Sprintf(
		"Masked view of %s. A field not listed as exposed is not available. Classification: %s.",
		origin, strings.Join(plan.notes, "; "),
	)

	if length := utf8.RuneCountInString(description); length > maxViewDescription {
		return View{}, fmt.Errorf("%s: the description is %d characters, over BigQuery's limit of %d", source.Table, length, maxViewDescription)
	}

	view := View{Columns: make([]string, len(columns))}

	definitions := make([]string, len(columns))
	selected := make([]string, len(columns))

	for index, column := range columns {
		if utf8.RuneCountInString(column.description) > maxColumnDescription {
			return View{}, fmt.Errorf("%s: the description of %s is over BigQuery's limit of %d characters", source.Table, column.name, maxColumnDescription)
		}

		view.Columns[index] = column.name
		definitions[index] = fmt.Sprintf("  %s OPTIONS(description=%s)", quote(column.name), stringLiteral(column.description))
		selected[index] = "  " + column.selected()
	}

	view.SQL = fmt.Sprintf("CREATE OR REPLACE VIEW %s (\n%s\n)\nOPTIONS(description=%s)\nAS SELECT\n%s\nFROM %s",
		name,
		strings.Join(definitions, ",\n"),
		stringLiteral(description),
		strings.Join(selected, ",\n"),
		quote(v.Project+"."+source.Dataset+"."+source.Table),
	)

	return view, nil
}

// column is one column of the view, or one field of a record in it.
type column struct {
	name, expression, description string
}

// selected is the column in a select list: its expression, named for it.
func (c column) selected() string {
	if c.expression == quote(c.name) {
		return c.expression
	}

	return c.expression + " AS " + quote(c.name)
}

// plan decides how each field of a schema is shown, and notes each decision for the
// view's description.
type plan struct {
	masks   map[string]Expr
	classes map[string]umbra.Class
	shown   map[string][]umbra.Exposure
	notes   []string
}

// newPlan plans the view of a table whose leaves have the given types.
func newPlan(table umbra.Table, leaves map[string]umbra.Type, masks map[string]Expr) *plan {
	p := &plan{masks: masks, classes: map[string]umbra.Class{}, shown: map[string][]umbra.Exposure{}}

	fitting := umbra.Table{Name: table.Name}

	for _, column := range table.Columns {
		p.classes[column.Name] = column.Class

		if leaf, exists := leaves[column.Name]; exists && column.CheckType(leaf) == nil {
			fitting.Columns = append(fitting.Columns, column)
		}
	}

	canMask := func(term string) bool {
		_, isMasked := masks[term]

		return isMasked
	}

	for _, exposure := range fitting.Exposures(canMask) {
		p.shown[exposure.Column] = append(p.shown[exposure.Column], exposure)
	}

	return p
}

// fields shows what it may of each field of a record, or of the table when parent is
// empty, and counts the fields it shows.
func (p *plan) fields(parent, prefix string, fields []Field, depth int) ([]column, int, error) {
	var columns []column

	shown := 0

	for _, field := range fields {
		reference := quote(field.Name)
		if parent != "" {
			reference = parent + "." + reference
		}

		var (
			fieldColumns []column
			err          error
		)

		if isRecord(field) {
			fieldColumns, err = p.record(reference, prefix+field.Name, field, depth)
		} else {
			fieldColumns = p.leaf(reference, prefix+field.Name, field)
		}

		if err != nil {
			return nil, 0, err
		}

		if len(fieldColumns) > 0 {
			shown++
		}

		columns = append(columns, fieldColumns...)
	}

	seen := map[string]bool{}

	for _, column := range columns {
		name := strings.ToLower(column.name)
		if seen[name] {
			return nil, 0, fmt.Errorf("two exposed fields are both named %s%s", prefix, column.name)
		}

		seen[name] = true
	}

	return columns, shown, nil
}

// leaf shows what the table's class allows of a field that isn't a record.
func (p *plan) leaf(reference, path string, field Field) []column {
	class, isClassified := p.classes[path]
	exposures := p.shown[path]

	switch {
	case !isClassified:
		p.notes = append(p.notes, path+": not classified, not exposed")

		return nil
	case len(exposures) == 0:
		p.notes = append(p.notes, fmt.Sprintf("%s: %s, not exposed", path, class))

		return nil
	}

	columns := make([]column, len(exposures))
	names := make([]string, len(exposures))

	for index, exposure := range exposures {
		expression := reference
		if exposure.Term != "" {
			expression = p.masks[exposure.Term](reference)
		}

		// The exposure's name is its column's path with a masker's suffix; within a record
		// it takes the field's own name instead.
		name := field.Name + strings.TrimPrefix(exposure.Name, exposure.Column)

		columns[index] = column{name: name, expression: expression, description: exposure.Label}
		names[index] = name
	}

	if _, isAllow := class.(umbra.Allow); isAllow {
		p.notes = append(p.notes, fmt.Sprintf("%s: %s, exposed", path, class))
	} else {
		p.notes = append(p.notes, fmt.Sprintf("%s: %s, exposed as %s", path, class, strings.Join(names, ", ")))
	}

	return columns
}

// record rebuilds a record from the fields it may show, keeping a NULL record NULL, or
// shows nothing when it may show none of them. A repeated record is rebuilt element by
// element.
func (p *plan) record(reference, path string, field Field, depth int) ([]column, error) {
	parent := reference
	element := fmt.Sprintf("element_%d", depth)

	if field.Repeated {
		parent = element
	}

	children, shown, err := p.fields(parent, path+".", field.Fields, depth+1)
	if err != nil || len(children) == 0 {
		return nil, err
	}

	selected := make([]string, len(children))
	for index, child := range children {
		selected[index] = child.expression + " AS " + quote(child.name)
	}

	expression := fmt.Sprintf("IF(%s IS NULL, NULL, STRUCT(%s))", reference, strings.Join(selected, ", "))
	if field.Repeated {
		expression = fmt.Sprintf("ARRAY(SELECT AS STRUCT %s FROM UNNEST(%s) AS %s)", strings.Join(selected, ", "), reference, element)
	}

	description := fmt.Sprintf("pii:record %d of %d fields exposed; the view description lists each field", shown, len(field.Fields))

	return []column{{name: field.Name, expression: expression, description: description}}, nil
}

// quote quotes a name, or a dotted path to one, with backticks.
func quote(name string) string {
	return "`" + strings.NewReplacer(`\`, `\\`, "`", "\\`").Replace(name) + "`"
}

// stringLiteral writes a double-quoted string.
func stringLiteral(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`).Replace(value) + `"`
}

// regexLiteral writes a pattern as a raw string when one can hold it, and otherwise as a
// quoted one.
func regexLiteral(pattern string) string {
	if strings.ContainsAny(pattern, "'\n\r") || strings.HasSuffix(pattern, `\`) {
		return stringLiteral(pattern)
	}

	return "r'" + pattern + "'"
}
