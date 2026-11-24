package umbra

import (
	"fmt"
	"io/fs"
	"iter"
	"maps"
	"slices"
	"strings"
	"unicode"
)

// key is the struct tag key of a class, and the prefix of its label in a view.
const key = "pii"

// A Catalog is the classification of every column of every table, by dataset.
// Build one with a [Registry], or [Load] one from its files.
type Catalog struct {
	datasets map[string]map[string]Table
}

// A Table is a table's columns, in order, and their classes.
type Table struct {
	Name    string
	Columns []Column
}

// A Column is one column of a table and its class.
type Column struct {
	Name  string
	Class Class
}

// An Exposure is one column of a masked view: a table column's value as it is, or what
// a masker derives from it.
type Exposure struct {
	// Column is the table column it shows.
	Column string
	// Name is the view column: the table column's name, with the masker's suffix if a
	// masker derives it.
	Name string
	// Term is the masker's term, or empty when the value shows as it is.
	Term string
	// Label is the tag that shows it, such as "pii:allow" or "pii:generalise=domain".
	Label string
}

// Table returns a table of the dataset.
func (c *Catalog) Table(dataset, name string) (Table, bool) {
	table, isFound := c.datasets[dataset][name]

	return table, isFound
}

// Tables returns the tables of a dataset, by name.
func (c *Catalog) Tables(dataset string) iter.Seq[Table] {
	return func(yield func(Table) bool) {
		tables := c.datasets[dataset]

		for _, name := range slices.Sorted(maps.Keys(tables)) {
			if !yield(tables[name]) {
				return
			}
		}
	}
}

// Exposures lists what a view of the table shows, in column order. can reports whether
// the view can compute a masker's term; a term it can't is left out, and a column with
// nothing left stays hidden. A nil can computes every term.
func (t Table) Exposures(can func(term string) bool) []Exposure {
	var exposures []Exposure

	for _, column := range t.Columns {
		switch class := column.Class.(type) {
		case Allow:
			exposures = append(exposures, Exposure{
				Column: column.Name,
				Name:   column.Name,
				Label:  key + ":" + class.String(),
			})
		case Mask:
			for _, masker := range class.Maskers {
				if can != nil && !can(masker.Term) {
					continue
				}

				exposures = append(exposures, Exposure{
					Column: column.Name,
					Name:   column.Name + "_" + masker.Suffix,
					Term:   masker.Term,
					Label:  key + ":" + masker.Term,
				})
			}
		}
	}

	return exposures
}

// add files a table under its dataset, unless the policy finds a problem with it.
func (c *Catalog) add(policy Policy, dataset string, table Table) []error {
	if _, exists := c.datasets[dataset][table.Name]; exists {
		return []error{fmt.Errorf("%s %s: declared twice", dataset, table.Name)}
	}

	if problems := policy.checkTable(dataset, table); len(problems) > 0 {
		return problems
	}

	if c.datasets == nil {
		c.datasets = map[string]map[string]Table{}
	}

	if c.datasets[dataset] == nil {
		c.datasets[dataset] = map[string]Table{}
	}

	c.datasets[dataset][table.Name] = table

	return nil
}

// checkTable reports what keeps a table out of a catalog: a name its file can't have,
// a column classified twice, a masker that reads a column the table doesn't allow, two
// columns shown under one name, and anything the policy's rules refuse.
func (p Policy) checkTable(dataset string, table Table) []error {
	if problems := checkNames(dataset, table); len(problems) > 0 {
		return problems
	}

	var problems []error

	allowed := map[string]bool{}
	for _, column := range table.Columns {
		_, allowed[column.Name] = column.Class.(Allow)
	}

	for _, column := range table.Columns {
		mask, isMask := column.Class.(Mask)
		if !isMask {
			continue
		}

		for _, masker := range mask.Maskers {
			for _, read := range masker.Reads {
				if !allowed[read] {
					problems = append(problems, fmt.Errorf(
						"%s %s.%s: %s reads %s, which the table must allow", dataset, table.Name, column.Name, masker.Term, read,
					))
				}
			}
		}
	}

	shown := map[string]bool{}

	for _, exposure := range table.Exposures(nil) {
		if shown[exposure.Name] {
			problems = append(problems, fmt.Errorf("%s %s: two columns are shown as %s", dataset, table.Name, exposure.Name))
		}

		shown[exposure.Name] = true
	}

	for _, rule := range p.Rules {
		if err := rule(dataset, table); err != nil {
			problems = append(problems, fmt.Errorf("%s %s: %w", dataset, table.Name, err))
		}
	}

	return problems
}

// checkNames reports a dataset or table that can't name a catalog file, and a column a
// catalog file can't hold or holds twice.
func checkNames(dataset string, table Table) []error {
	switch {
	case !fs.ValidPath(dataset) || dataset == ".":
		return []error{fmt.Errorf("dataset %q can't name a directory", dataset)}
	case !fs.ValidPath(table.Name) || strings.Contains(table.Name, "/") || table.Name == ".":
		return []error{fmt.Errorf("%s: table %q can't name a file", dataset, table.Name)}
	case len(table.Columns) == 0:
		return []error{fmt.Errorf("%s %s: no columns", dataset, table.Name)}
	}

	var problems []error

	seen := map[string]bool{}

	for _, column := range table.Columns {
		switch {
		case column.Name == "" || strings.HasPrefix(column.Name, "#") || strings.ContainsFunc(column.Name, unicode.IsSpace):
			problems = append(problems, fmt.Errorf("%s %s: column %q can't be written to a catalog file", dataset, table.Name, column.Name))
		case seen[column.Name]:
			problems = append(problems, fmt.Errorf("%s %s.%s: classified twice", dataset, table.Name, column.Name))
		}

		seen[column.Name] = true
	}

	return problems
}
