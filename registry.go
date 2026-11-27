package umbra

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// A Registry lists the tables of each dataset, and builds a [Catalog] of them.
type Registry struct {
	policy   Policy
	mapper   Mapper
	datasets []*Dataset
}

// A Dataset lists the tables of one place that holds them, such as a database.
type Dataset struct {
	name   string
	mapper Mapper
	tables []declaration
}

// declaration is a table as it was registered: the model that stores it, if any, and the
// tags given in code.
type declaration struct {
	name  string
	model reflect.Type
	tags  []Tagged
}

// A Mapper lists the columns a model is stored in.
type Mapper func(model reflect.Type) ([]Field, error)

// A Field is a column, and the struct field that stores it.
type Field struct {
	Column string
	Struct reflect.StructField
}

// Tagged is a column's tag given in code, for a column no struct field stores, or one
// whose field can't carry a tag, such as a field of an embedded type from another package.
type Tagged struct {
	Column, Tag string
}

// NewRegistry returns an empty registry whose tags use the policy's vocabulary.
// mapper names the columns of each model; nil means [SnakeCase]. [Dataset.MapWith] gives a
// dataset a mapper of its own.
func NewRegistry(policy Policy, mapper Mapper) *Registry {
	if mapper == nil {
		mapper = SnakeCase
	}

	return &Registry{policy: policy, mapper: mapper}
}

// Tag classifies a column with a tag given in code.
func Tag(column, tag string) Tagged {
	return Tagged{Column: column, Tag: tag}
}

// Dataset returns the named dataset, adding it on first use.
func (r *Registry) Dataset(name string) *Dataset {
	for _, dataset := range r.datasets {
		if dataset.name == name {
			return dataset
		}
	}

	dataset := &Dataset{name: name}
	r.datasets = append(r.datasets, dataset)

	return dataset
}

// MapWith names the columns of the dataset's models with mapper, in place of the registry's,
// and returns the dataset.
func (d *Dataset) MapWith(mapper Mapper) *Dataset {
	d.mapper = mapper

	return d
}

// Model registers the table T is stored in, classified by the pii tags on T's fields and
// by extra tags. An extra tag classifies a column no field stores, or the column of a field
// with no tag, in that field's place. A field tagged pii:"-" is not a column, whatever the
// mapper says.
func Model[T any](dataset *Dataset, table string, extra ...Tagged) {
	dataset.tables = append(dataset.tables, declaration{name: table, model: reflect.TypeFor[T](), tags: extra})
}

// Columns registers a table no model stores, classified by tags given in code.
func (d *Dataset) Columns(table string, columns ...Tagged) {
	d.tables = append(d.tables, declaration{name: table, tags: columns})
}

// Catalog classifies every registered column and checks each table against the policy.
// It reports every problem at once, each naming its column and, for a model, its field.
func (r *Registry) Catalog() (*Catalog, error) {
	if err := r.policy.check(); err != nil {
		return nil, err
	}

	catalog := &Catalog{}

	var problems []error

	for _, dataset := range r.datasets {
		for _, declared := range dataset.tables {
			table, tableProblems := r.table(dataset, declared)
			if len(tableProblems) == 0 {
				tableProblems = catalog.add(r.policy, dataset.name, table)
			}

			problems = append(problems, tableProblems...)
		}
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	return catalog, nil
}

// table classifies the columns of a declared table, from its model's fields, then its
// tags given in code that no field took.
func (r *Registry) table(dataset *Dataset, declared declaration) (Table, []error) {
	table := Table{Name: declared.name}

	mapper := r.mapper
	if dataset.mapper != nil {
		mapper = dataset.mapper
	}

	var problems []error

	taken := make([]bool, len(declared.tags))

	if declared.model != nil {
		model := declared.model
		for model.Kind() == reflect.Pointer {
			model = model.Elem()
		}

		typeName := model.Name()
		if typeName == "" {
			typeName = model.String()
		}

		fields, err := mapper(model)
		if err != nil {
			return table, []error{fmt.Errorf("%s %s (%s): %w", dataset.name, declared.name, typeName, err)}
		}

		for _, field := range fields {
			where := fmt.Sprintf("%s %s.%s (%s.%s)", dataset.name, declared.name, field.Column, typeName, field.Struct.Name)

			tag, isTagged := field.Struct.Tag.Lookup(key)

			switch {
			case tag == "-":
				continue
			case !isTagged:
				index := slices.IndexFunc(declared.tags, func(tagged Tagged) bool { return tagged.Column == field.Column })
				if index < 0 {
					problems = append(problems, fmt.Errorf("%s: a column with no %s tag", where, key))

					continue
				}

				taken[index] = true
				tag = declared.tags[index].Tag
			}

			class, err := ParseTag(tag, r.policy)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", where, err))

				continue
			}

			table.Columns = append(table.Columns, Column{Name: field.Column, Class: class})
		}
	}

	for index, tagged := range declared.tags {
		if taken[index] {
			continue
		}

		class, err := ParseTag(tagged.Tag, r.policy)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s %s.%s: %w", dataset.name, declared.name, tagged.Column, err))

			continue
		}

		table.Columns = append(table.Columns, Column{Name: tagged.Column, Class: class})
	}

	return table, problems
}

// SnakeCase is a [Mapper] that stores each exported field in a column named in snake
// case: OrgID in org_id, and HTTPServer in http_server. It looks through embedded
// structs.
func SnakeCase(model reflect.Type) ([]Field, error) {
	if model.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", model)
	}

	var fields []Field

	for _, field := range reflect.VisibleFields(model) {
		switch {
		case !field.IsExported():
			continue
		case field.Anonymous && isStruct(field.Type):
			continue // its fields are visible on their own
		}

		fields = append(fields, Field{Column: snakeCase(field.Name), Struct: field})
	}

	return fields, nil
}

func isStruct(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Kind() == reflect.Struct
}

// snakeCase starts a word at each upper-case letter, unless it continues an initialism.
func snakeCase(name string) string {
	runes := []rune(name)

	var out strings.Builder

	for index, r := range runes {
		if index > 0 && unicode.IsUpper(r) {
			previous := runes[index-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])

			if !unicode.IsUpper(previous) || nextIsLower {
				out.WriteByte('_')
			}
		}

		out.WriteRune(unicode.ToLower(r))
	}

	return out.String()
}
