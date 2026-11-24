package postgres

import (
	"testing"

	"github.com/feds01/umbra"
)

func TestTypeOfSortsATypeByItsKind(t *testing.T) {
	tests := map[string]umbra.Type{
		"text":                        {Name: "text", Kind: umbra.Text},
		"citext":                      {Name: "citext", Kind: umbra.Text},
		"character varying(255)":      {Name: "character varying(255)", Kind: umbra.Text},
		"character(1)":                {Name: "character(1)", Kind: umbra.Text},
		"text[]":                      {Name: "text[]", Kind: umbra.Text, Array: true},
		"uuid":                        {Name: "uuid", Kind: umbra.UUID},
		"inet":                        {Name: "inet", Kind: umbra.Inet},
		"enum":                        {Name: "enum", Kind: umbra.Scalar},
		"numeric(10,2)":               {Name: "numeric(10,2)", Kind: umbra.Scalar},
		"timestamp(3) with time zone": {Name: "timestamp(3) with time zone", Kind: umbra.Scalar},
		"jsonb":                       {Name: "jsonb"},
		"bytea":                       {Name: "bytea"},
		"vector(1536)":                {Name: "vector(1536)"},
		"document_holder":             {Name: "document_holder"},
		"document[]":                  {Name: "document[]", Array: true},
		"other.vector(3)":             {Name: "other.vector(3)"},
	}

	for name, want := range tests {
		if got := typeOf(name); got != want {
			t.Errorf("%s: got %+v, want %+v", name, got, want)
		}
	}
}
