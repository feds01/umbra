package umbra

import (
	"errors"
	"fmt"
	"maps"
)

// A Schema is what a dataset really holds: the type of each column, by table.
type Schema map[string]map[string]Type

// A Type is a column's type in the database.
type Type struct {
	// Name is the type as the database spells it, such as "character varying(255)".
	Name string
	// Kind is the kind of value it holds. A reader that doesn't know a type leaves it Opaque.
	Kind Kind
	// Array marks a list of values, which no masker reads.
	Array bool
}

// The problems [Catalog.Check] reports each wrap one of these.
var (
	ErrUnclassified = errors.New("not classified")
	ErrStale        = errors.New("not in the schema")
	ErrType         = errors.New("class does not fit the type")
)

// Check compares a dataset's tables in the catalog with the schema the database really
// has. It reports each table or column the catalog doesn't classify, each one it
// classifies that the schema doesn't have, and each column whose class doesn't fit its type.
func (c *Catalog) Check(dataset string, actual Schema) error {
	classified := c.datasets[dataset]

	var problems []error

	for _, name := range union(maps.Keys(actual), maps.Keys(classified)) {
		table, isClassified := classified[name]
		columns, exists := actual[name]

		switch {
		case !isClassified:
			problems = append(problems, fmt.Errorf("%s %s: %w", dataset, name, ErrUnclassified))
		case !exists:
			problems = append(problems, fmt.Errorf("%s %s: %w", dataset, name, ErrStale))
		default:
			problems = append(problems, checkColumns(dataset, table, columns)...)
		}
	}

	return errors.Join(problems...)
}

func checkColumns(dataset string, table Table, actual map[string]Type) []error {
	classified := map[string]Column{}
	for _, column := range table.Columns {
		classified[column.Name] = column
	}

	var problems []error

	for _, name := range union(maps.Keys(actual), maps.Keys(classified)) {
		column, isClassified := classified[name]
		columnType, exists := actual[name]
		where := dataset + " " + table.Name + "." + name

		switch {
		case !isClassified:
			problems = append(problems, fmt.Errorf("%s: %w", where, ErrUnclassified))
		case !exists:
			problems = append(problems, fmt.Errorf("%s: %w", where, ErrStale))
		default:
			if err := column.CheckType(columnType); err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", where, err))
			}
		}
	}

	return problems
}

// CheckType reports a type the column's class doesn't fit, with an error that wraps
// [ErrType]. Allow fits an Opaque type only with a basis, and a masker reads only a single
// value of a kind it knows.
func (c Column) CheckType(t Type) error {
	switch class := c.Class.(type) {
	case Allow:
		if t.Kind == Opaque && class.Basis == "" {
			return typeError(t.Name + " can hold anything: deny it, or allow it with a basis")
		}
	case Mask:
		for _, masker := range class.Maskers {
			if t.Array || t.Kind&masker.Kinds == 0 {
				return typeError(fmt.Sprintf("%s reads a single %s value, not %s", masker.Term, masker.Kinds, t.Name))
			}
		}
	}

	return nil
}

// typeError says why a class doesn't fit a type. It wraps ErrType without repeating it.
type typeError string

func (e typeError) Error() string { return string(e) }

func (typeError) Unwrap() error { return ErrType }
