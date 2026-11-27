package bigquery

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/feds01/umbra"
)

// Columns is an [umbra.Mapper] for the row structs of BigQuery's Go client. A field's column
// is the name its bigquery tag gives, or else the field's own name, and a field tagged
// bigquery:"-" is not a column.
//
// A record is a field whose struct, through pointers, slices and arrays, names one of its
// fields with a bigquery tag. Its columns are its leaves, named by dotted path: the field
// Name of the record City of the record Geolocation is geolocation.city.name, repeated or
// not. Any other struct, such as time.Time, is a single column, as is a record tagged
// with the json option. The fields of an embedded struct are the outer struct's own.
func Columns(model reflect.Type) ([]umbra.Field, error) {
	if model.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", model)
	}

	return leaves(model, "", nil)
}

// leaves lists the columns of a struct whose fields are named under prefix. within holds
// the records being listed, so that a recursive one is reported rather than followed.
func leaves(model reflect.Type, prefix string, within []reflect.Type) ([]umbra.Field, error) {
	if slices.Contains(within, model) {
		return nil, fmt.Errorf("%s: %s is recursive", strings.TrimSuffix(prefix, "."), model)
	}

	var fields []umbra.Field

	for _, field := range reflect.VisibleFields(model) {
		name, isJSON := nameOf(field)

		switch {
		case !field.IsExported() || name == "-":
			continue
		case field.Anonymous && isStruct(field.Type):
			continue // its fields are visible on their own
		}

		path := prefix + name

		record, isRecord := recordOf(field.Type)
		if !isRecord || isJSON {
			fields = append(fields, umbra.Field{Column: path, Struct: field})

			continue
		}

		nested, err := leaves(record, path+".", append(slices.Clip(within), model))
		if err != nil {
			return nil, err
		}

		fields = append(fields, nested...)
	}

	return fields, nil
}

// nameOf reads a field's bigquery tag: its column's name, and whether the json option
// stores the field as one JSON column.
func nameOf(field reflect.StructField) (string, bool) {
	name, options, _ := strings.Cut(field.Tag.Get("bigquery"), ",")
	if name == "" {
		name = field.Name
	}

	return name, slices.Contains(strings.Split(options, ","), "json")
}

// recordOf returns the struct a field's type holds, through pointers, slices and arrays,
// when that struct names one of its fields with a bigquery tag.
func recordOf(t reflect.Type) (reflect.Type, bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, false
	}

	for _, field := range reflect.VisibleFields(t) {
		if _, isTagged := field.Tag.Lookup("bigquery"); isTagged {
			return t, true
		}
	}

	return nil, false
}

func isStruct(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Kind() == reflect.Struct
}
