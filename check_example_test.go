package umbra_test

import (
	"errors"
	"fmt"
	"log"
	"testing/fstest"

	"github.com/feds01/umbra"
)

func ExampleCatalog_Check() {
	catalog, err := umbra.Load(fstest.MapFS{
		"postgres/contacts.pii": {Data: []byte("id allow\norg_id allow\nname deny=person\nemail generalise=domain\nnotes deny=free_text\n")},
	}, policy())
	if err != nil {
		log.Fatal(err)
	}

	err = catalog.Check("postgres", umbra.Schema{
		"contacts": {
			"id":       {Name: "uuid", Kind: umbra.UUID},
			"org_id":   {Name: "uuid", Kind: umbra.UUID},
			"name":     {Name: "text", Kind: umbra.Text},
			"email":    {Name: "text[]", Kind: umbra.Text, Array: true},
			"settings": {Name: "jsonb"}, // a Kind of 0 is Opaque
		},
		"users": {
			"id": {Name: "uuid", Kind: umbra.UUID},
		},
	})

	fmt.Println(err)
	fmt.Println(errors.Is(err, umbra.ErrUnclassified))

	// Output:
	// postgres contacts.email: generalise=domain reads a single text value, not text[]
	// postgres contacts.notes: not in the schema
	// postgres contacts.settings: not classified
	// postgres users: not classified
	// true
}
