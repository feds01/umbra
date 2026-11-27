package bigquery_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/feds01/umbra/bigquery"
)

func TestColumnsNamesLeavesByDottedPath(t *testing.T) {
	type City struct {
		Name string `bigquery:"name"`
	}

	type Subdivision struct {
		ISOCode string `bigquery:"iso_code"`
	}

	type Geolocation struct {
		City         *City         `bigquery:"city,nullable"`
		Subdivisions []Subdivision `bigquery:"subdivisions"`
	}

	type Audit struct {
		InsertedAt time.Time `bigquery:"inserted_at"`
	}

	type Event struct {
		ID          string       `bigquery:"id"`
		Geolocation *Geolocation `bigquery:"geolocation,nullable"`
		Body        Geolocation  `bigquery:"body,json"`
		Internal    string       `bigquery:"-"`
		Channel     string
		Audit
	}

	fields, err := bigquery.Columns(reflect.TypeFor[Event]())
	if err != nil {
		t.Fatal(err)
	}

	var columns, structFields []string
	for _, field := range fields {
		columns = append(columns, field.Column)
		structFields = append(structFields, field.Struct.Name)
	}

	wantColumns := []string{"id", "geolocation.city.name", "geolocation.subdivisions.iso_code", "body", "Channel", "inserted_at"}
	if !reflect.DeepEqual(columns, wantColumns) {
		t.Errorf("got columns %v, want %v", columns, wantColumns)
	}

	wantFields := []string{"ID", "Name", "ISOCode", "Body", "Channel", "InsertedAt"}
	if !reflect.DeepEqual(structFields, wantFields) {
		t.Errorf("got fields %v, want %v", structFields, wantFields)
	}
}

func TestColumnsRefusesWhatItCannotMap(t *testing.T) {
	type Node struct {
		Value    string  `bigquery:"value"`
		Children []*Node `bigquery:"children"`
	}

	if _, err := bigquery.Columns(reflect.TypeFor[Node]()); err == nil || !strings.HasPrefix(err.Error(), "children: ") {
		t.Errorf("got %v for a recursive record", err)
	}

	if _, err := bigquery.Columns(reflect.TypeFor[string]()); err == nil || err.Error() != "string is not a struct" {
		t.Errorf("got %v for a string", err)
	}
}
