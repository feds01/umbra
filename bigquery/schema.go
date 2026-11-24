package bigquery

import (
	"strings"

	"github.com/feds01/umbra"
)

// Flatten lists the leaves of a schema, the fields that aren't records, by dotted path,
// such as properties.email, with their types, for [umbra.Catalog.Check].
func Flatten(fields []Field) map[string]umbra.Type {
	leaves := map[string]umbra.Type{}
	flatten(leaves, "", fields)

	return leaves
}

func flatten(leaves map[string]umbra.Type, prefix string, fields []Field) {
	for _, field := range fields {
		if isRecord(field) {
			flatten(leaves, prefix+field.Name+".", field.Fields)

			continue
		}

		leaves[prefix+field.Name] = typeOf(field)
	}
}

func isRecord(field Field) bool {
	return strings.EqualFold(field.Type, "RECORD") || strings.EqualFold(field.Type, "STRUCT")
}

// typeOf sorts a field by the kind of value it holds. A type it doesn't know, such as
// JSON, BYTES or GEOGRAPHY, is Opaque.
func typeOf(field Field) umbra.Type {
	name := field.Type
	if field.Repeated {
		name = "ARRAY<" + name + ">"
	}

	return umbra.Type{Name: name, Kind: kindOf(strings.ToUpper(field.Type)), Array: field.Repeated}
}

func kindOf(name string) umbra.Kind {
	switch name {
	case "STRING":
		return umbra.Text
	case "INTEGER", "INT64", "FLOAT", "FLOAT64", "NUMERIC", "BIGNUMERIC", "BOOLEAN", "BOOL",
		"TIMESTAMP", "DATE", "TIME", "DATETIME", "INTERVAL":
		return umbra.Scalar
	}

	return umbra.Opaque
}
