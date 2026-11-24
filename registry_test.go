package umbra_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/feds01/umbra"
)

func TestSnakeCaseNamesAColumnAfterItsField(t *testing.T) {
	tests := map[string]string{
		"ID":           "id",
		"OrgID":        "org_id",
		"CreatedAt":    "created_at",
		"HTTPServer":   "http_server",
		"SignInURL":    "sign_in_url",
		"Address2Line": "address2_line",
		"A":            "a",
	}

	for field, want := range tests {
		t.Run(field, func(t *testing.T) {
			model := reflect.StructOf([]reflect.StructField{{Name: field, Type: reflect.TypeFor[string]()}})

			fields, err := umbra.SnakeCase(model)
			if err != nil {
				t.Fatal(err)
			}

			if len(fields) != 1 || fields[0].Column != want {
				t.Errorf("got %+v, want %s", fields, want)
			}
		})
	}
}

func TestSnakeCaseFindsTheColumnsOfAStruct(t *testing.T) {
	type Timestamps struct {
		CreatedAt string
	}

	type owner struct {
		OwnerID string
	}

	type Row struct {
		ID string
		*Timestamps
		owner
		//lint:ignore U1000 SnakeCase must skip an unexported field.
		secret string
	}

	fields, err := umbra.SnakeCase(reflect.TypeFor[Row]())
	if err != nil {
		t.Fatal(err)
	}

	var columns []string
	for _, field := range fields {
		columns = append(columns, field.Column)
	}

	if want := []string{"id", "created_at", "owner_id"}; !reflect.DeepEqual(columns, want) {
		t.Errorf("got %v, want %v", columns, want)
	}

	if _, err := umbra.SnakeCase(reflect.TypeFor[string]()); err == nil || err.Error() != "string is not a struct" {
		t.Errorf("got %v for a string", err)
	}
}

func TestCatalogRefusesABrokenTable(t *testing.T) {
	type Row struct {
		ID string `pii:"allow"`
	}

	tests := map[string]struct {
		register func(*umbra.Registry)
		want     string
	}{
		"a table declared twice": {
			func(registry *umbra.Registry) {
				registry.Dataset("postgres").Columns("t", umbra.Tag("id", "allow"))
				registry.Dataset("postgres").Columns("t", umbra.Tag("id", "allow"))
			},
			"postgres t: declared twice",
		},
		"a column classified twice": {
			func(registry *umbra.Registry) {
				umbra.Model[Row](registry.Dataset("postgres"), "t", umbra.Tag("id", "deny=internal"))
			},
			"postgres t.id: classified twice",
		},
		"a table with no columns": {
			func(registry *umbra.Registry) { registry.Dataset("postgres").Columns("t") },
			"postgres t: no columns",
		},
		"a dataset no directory can be named": {
			func(registry *umbra.Registry) { registry.Dataset("../up").Columns("t", umbra.Tag("id", "allow")) },
			`dataset "../up" can't name a directory`,
		},
		"a table no file can be named": {
			func(registry *umbra.Registry) { registry.Dataset("postgres").Columns("a/b", umbra.Tag("id", "allow")) },
			`postgres: table "a/b" can't name a file`,
		},
		"a column a catalog file can't hold": {
			func(registry *umbra.Registry) { registry.Dataset("postgres").Columns("t", umbra.Tag("a b", "allow")) },
			`postgres t: column "a b" can't be written to a catalog file`,
		},
		"a column shown under another's name": {
			func(registry *umbra.Registry) {
				registry.Dataset("postgres").Columns("t",
					umbra.Tag("org_id", "allow"),
					umbra.Tag("ip", "pseudonymise"),
					umbra.Tag("ip_pseudonym", "allow"),
				)
			},
			"postgres t: two columns are shown as ip_pseudonym",
		},
		"a masker reading a denied column": {
			func(registry *umbra.Registry) {
				registry.Dataset("postgres").Columns("t", umbra.Tag("org_id", "deny=internal"), umbra.Tag("ip", "pseudonymise"))
			},
			"postgres t.ip: pseudonymise reads org_id, which the table must allow",
		},
		"a model that isn't a struct": {
			func(registry *umbra.Registry) { umbra.Model[int](registry.Dataset("postgres"), "t") },
			"postgres t (int): int is not a struct",
		},
		"a bad tag given in code": {
			func(registry *umbra.Registry) { registry.Dataset("postgres").Columns("t", umbra.Tag("id", "")) },
			"postgres t.id: empty tag",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			registry := umbra.NewRegistry(policy(), nil)
			test.register(registry)

			_, err := registry.Catalog()
			if err == nil || err.Error() != test.want {
				t.Errorf("got %v, want %s", err, test.want)
			}
		})
	}
}

func TestCatalogAsksTheMapperForAModelsColumns(t *testing.T) {
	type Contact struct {
		Email string `pii:"deny=person"`
	}

	pointed := func(model reflect.Type) ([]umbra.Field, error) {
		field, _ := model.FieldByName("Email")

		return []umbra.Field{{Column: "email_address", Struct: field}}, nil
	}

	registry := umbra.NewRegistry(policy(), pointed)
	umbra.Model[*Contact](registry.Dataset("postgres"), "contacts")

	catalog, err := registry.Catalog()
	if err != nil {
		t.Fatal(err)
	}

	contacts, _ := catalog.Table("postgres", "contacts")
	if want := []umbra.Column{{Name: "email_address", Class: umbra.Deny{Reason: "person"}}}; !reflect.DeepEqual(contacts.Columns, want) {
		t.Errorf("got %v, want %v", contacts.Columns, want)
	}

	failing := umbra.NewRegistry(policy(), func(reflect.Type) ([]umbra.Field, error) { return nil, errors.New("no schema") })
	umbra.Model[Contact](failing.Dataset("postgres"), "contacts")

	if _, err := failing.Catalog(); err == nil || !strings.HasSuffix(err.Error(), "(Contact): no schema") {
		t.Errorf("got %v", err)
	}
}

func TestCatalogSkipsAFieldTaggedDashWhateverTheMapper(t *testing.T) {
	type Campaign struct {
		ID     string `pii:"allow"`
		Status string `pii:"-"` // computed by a query, not stored
	}

	everyField := func(model reflect.Type) ([]umbra.Field, error) {
		var fields []umbra.Field
		for _, field := range reflect.VisibleFields(model) {
			fields = append(fields, umbra.Field{Column: strings.ToLower(field.Name), Struct: field})
		}

		return fields, nil
	}

	registry := umbra.NewRegistry(policy(), everyField)
	umbra.Model[Campaign](registry.Dataset("postgres"), "campaigns")

	catalog, err := registry.Catalog()
	if err != nil {
		t.Fatal(err)
	}

	campaigns, _ := catalog.Table("postgres", "campaigns")
	if want := []umbra.Column{{Name: "id", Class: umbra.Allow{}}}; !reflect.DeepEqual(campaigns.Columns, want) {
		t.Errorf("got %v, want %v", campaigns.Columns, want)
	}
}

func TestATagGivenInCodeClassifiesAFieldWithNoTag(t *testing.T) {
	type Model struct { // an ORM's base type, whose fields can't be tagged
		ID        string
		CreatedAt string `pii:"allow"`
	}

	type Connection struct {
		Model
		Token string `pii:"deny=credential"`
	}

	registry := umbra.NewRegistry(policy(), nil)
	umbra.Model[Connection](registry.Dataset("postgres"), "connections",
		umbra.Tag("id", "allow"),
		umbra.Tag("legacy_state", "deny=internal"),
	)

	catalog, err := registry.Catalog()
	if err != nil {
		t.Fatal(err)
	}

	connections, _ := catalog.Table("postgres", "connections")

	want := []umbra.Column{
		{Name: "id", Class: umbra.Allow{}},
		{Name: "created_at", Class: umbra.Allow{}},
		{Name: "token", Class: umbra.Deny{Reason: "credential"}},
		{Name: "legacy_state", Class: umbra.Deny{Reason: "internal"}},
	}
	if !reflect.DeepEqual(connections.Columns, want) {
		t.Errorf("got %v, want %v", connections.Columns, want)
	}

	twice := umbra.NewRegistry(policy(), nil)
	umbra.Model[Connection](twice.Dataset("postgres"), "connections", umbra.Tag("id", "allow"), umbra.Tag("token", "allow"))

	if _, err := twice.Catalog(); err == nil || err.Error() != "postgres connections.token: classified twice" {
		t.Errorf("got %v for a tag beside a tagged field", err)
	}
}

func TestCatalogRefusesAnUnusablePolicy(t *testing.T) {
	text := umbra.Masker{Term: "hash", Suffix: "hash", Kinds: umbra.Text}
	reasons := []umbra.Reason{"person"}

	tests := map[string]struct {
		policy umbra.Policy
		want   string
	}{
		"no reasons": {
			umbra.Policy{},
			"policy: no reason to deny a column with",
		},
		"a reason of two words": {
			umbra.Policy{Reasons: []umbra.Reason{"free text"}},
			`policy: reason "free text" must be one word`,
		},
		"a reason twice": {
			umbra.Policy{Reasons: []umbra.Reason{"person", "person"}},
			`policy: reason "person" appears twice`,
		},
		"an empty basis": {
			umbra.Policy{Reasons: reasons, Bases: []umbra.Basis{""}},
			`policy: basis "" must be one word`,
		},
		"a term with a comma": {
			umbra.Policy{Reasons: reasons, Maskers: []umbra.Masker{{Term: "a,b", Suffix: "ab", Kinds: umbra.Text}}},
			`policy: term "a,b" must be one word`,
		},
		"a term twice": {
			umbra.Policy{Reasons: reasons, Maskers: []umbra.Masker{text, {Term: "hash", Suffix: "digest", Kinds: umbra.Text}}},
			`policy: term "hash" appears twice`,
		},
		"a suffix twice": {
			umbra.Policy{Reasons: reasons, Maskers: []umbra.Masker{text, {Term: "digest", Suffix: "hash", Kinds: umbra.Text}}},
			`policy: suffix "hash" appears twice`,
		},
		"a term that reads as allow": {
			umbra.Policy{Reasons: reasons, Maskers: []umbra.Masker{{Term: "allow=all", Suffix: "all", Kinds: umbra.Text}}},
			`policy: term "allow=all" reads as allow or deny`,
		},
		"a masker that reads nothing": {
			umbra.Policy{Reasons: reasons, Maskers: []umbra.Masker{{Term: "hash", Suffix: "hash"}}},
			"policy: hash reads no kind of value",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := umbra.NewRegistry(test.policy, nil).Catalog()
			if err == nil || err.Error() != test.want {
				t.Errorf("got %v, want %s", err, test.want)
			}
		})
	}
}

func TestKindNamesItsMembers(t *testing.T) {
	tests := map[umbra.Kind]string{
		umbra.Opaque:                         "opaque",
		umbra.Text:                           "text",
		umbra.Text | umbra.UUID:              "text or uuid",
		umbra.Text | umbra.UUID | umbra.Inet: "text, uuid or inet",
		umbra.Scalar:                         "scalar",
	}

	for kind, want := range tests {
		if kind.String() != want {
			t.Errorf("got %s, want %s", kind, want)
		}
	}
}
