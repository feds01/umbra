package umbra_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/feds01/umbra"
)

func TestCheckTypeRefusesAClassThatDoesNotFit(t *testing.T) {
	var (
		allow        = umbra.Allow{}
		allowJSON    = umbra.Allow{Basis: "business_data"}
		deny         = umbra.Deny{Reason: "json"}
		domain       = masks("generalise=domain")
		pseudonymise = masks("pseudonymise")
		both         = masks("generalise=domain", "pseudonymise")

		text         = umbra.Type{Name: "text", Kind: umbra.Text}
		textArray    = umbra.Type{Name: "text[]", Kind: umbra.Text, Array: true}
		varchar      = umbra.Type{Name: "character varying(255)", Kind: umbra.Text}
		uuid         = umbra.Type{Name: "uuid", Kind: umbra.UUID}
		inet         = umbra.Type{Name: "inet", Kind: umbra.Inet}
		integer      = umbra.Type{Name: "integer", Kind: umbra.Scalar}
		timestamp    = umbra.Type{Name: "timestamp with time zone", Kind: umbra.Scalar}
		jsonb        = umbra.Type{Name: "jsonb"}
		documents    = umbra.Type{Name: "document[]", Array: true}
		bigQueryJSON = umbra.Type{Name: "JSON"}
	)

	tests := map[string]struct {
		class umbra.Class
		given umbra.Type
		want  string
	}{
		"allowed text":            {allow, text, ""},
		"allowed text array":      {allow, textArray, ""},
		"allowed timestamp":       {allow, timestamp, ""},
		"allowed json":            {allow, jsonb, "jsonb can hold anything: deny it, or allow it with a basis"},
		"allowed BigQuery JSON":   {allow, bigQueryJSON, "JSON can hold anything: deny it, or allow it with a basis"},
		"allowed array of a type": {allow, documents, "document[] can hold anything: deny it, or allow it with a basis"},
		"json with a basis":       {allowJSON, jsonb, ""},
		"denied json":             {deny, jsonb, ""},
		"generalised varchar":     {domain, varchar, ""},
		"generalised json":        {domain, jsonb, "generalise=domain reads a single text value, not jsonb"},
		"generalised number":      {domain, integer, "generalise=domain reads a single text value, not integer"},
		"generalised text array":  {domain, textArray, "generalise=domain reads a single text value, not text[]"},
		"pseudonymised uuid":      {pseudonymise, uuid, ""},
		"pseudonymised inet":      {pseudonymise, inet, ""},
		"pseudonymised timestamp": {pseudonymise, timestamp, "pseudonymise reads a single text, uuid or inet value, not timestamp with time zone"},
		"two maskers, one misfit": {both, uuid, "generalise=domain reads a single text value, not uuid"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := umbra.Column{Name: "c", Class: test.class}.CheckType(test.given)

			switch {
			case test.want == "" && err != nil:
				t.Errorf("got %v", err)
			case test.want != "" && (err == nil || err.Error() != test.want):
				t.Errorf("got %v, want %s", err, test.want)
			case test.want != "" && !errors.Is(err, umbra.ErrType):
				t.Errorf("%v does not wrap ErrType", err)
			}
		})
	}
}

func TestCheckReportsEachDifferenceOnce(t *testing.T) {
	catalog, err := umbra.Load(fstest.MapFS{
		"postgres/contacts.pii": {Data: []byte("id allow\norg_id allow\nemail generalise=domain,pseudonymise\ntoken deny=credential\n")},
		"postgres/legacy.pii":   {Data: []byte("id allow\n")},
	}, policy())
	if err != nil {
		t.Fatal(err)
	}

	uuid := umbra.Type{Name: "uuid", Kind: umbra.UUID}
	text := umbra.Type{Name: "text", Kind: umbra.Text}

	matching := umbra.Schema{
		"contacts": {"id": uuid, "org_id": uuid, "email": text, "token": text},
		"legacy":   {"id": uuid},
	}

	if err := catalog.Check("postgres", matching); err != nil {
		t.Errorf("a matching schema fails: %v", err)
	}

	drifted := umbra.Schema{
		"contacts": {"id": {Name: "jsonb"}, "org_id": uuid, "email": text, "added": text},
		"fresh":    {"id": uuid},
	}

	err = catalog.Check("postgres", drifted)

	want := "postgres contacts.added: not classified\n" +
		"postgres contacts.id: jsonb can hold anything: deny it, or allow it with a basis\n" +
		"postgres contacts.token: not in the schema\n" +
		"postgres fresh: not classified\n" +
		"postgres legacy: not in the schema"

	if err == nil || err.Error() != want {
		t.Fatalf("got\n%v\nwant\n%s", err, want)
	}

	for _, sentinel := range []error{umbra.ErrUnclassified, umbra.ErrStale, umbra.ErrType} {
		if !errors.Is(err, sentinel) {
			t.Errorf("does not wrap %v", sentinel)
		}
	}
}
