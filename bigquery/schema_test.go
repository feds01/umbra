package bigquery

import "testing"

func TestParseSchemaReadsAModeInAnyCase(t *testing.T) {
	for _, mode := range []string{"REPEATED", "repeated"} {
		fields, err := ParseSchema([]byte(`[{"name": "tags", "type": "STRING", "mode": "` + mode + `"}]`))
		if err != nil {
			t.Fatal(err)
		}

		if !fields[0].Repeated {
			t.Errorf("mode %q isn't repeated", mode)
		}
	}
}

func TestParseSchemaReadsNestedRecords(t *testing.T) {
	fields, err := ParseSchema([]byte(`[{"name": "a", "type": "RECORD", "fields": [{"name": "b", "type": "RECORD", "mode": "REPEATED", "fields": [{"name": "c", "type": "INTEGER"}]}]}]`))
	if err != nil {
		t.Fatal(err)
	}

	leaf := fields[0].Fields[0].Fields[0]

	if leaf.Name != "c" || leaf.Type != "INTEGER" || !fields[0].Fields[0].Repeated {
		t.Errorf("got %+v", fields)
	}
}

func TestParseSchemaRejectsWhatIsNotAList(t *testing.T) {
	if _, err := ParseSchema([]byte(`{"name": "id"}`)); err == nil {
		t.Error("an object parsed as a schema")
	}
}
