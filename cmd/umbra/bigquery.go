package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/feds01/umbra"
	bq "github.com/feds01/umbra/bigquery"
)

// schemaField is a field as `bq show --schema --format=json` writes it.
type schemaField struct {
	Name   string        `json:"name"`
	Type   string        `json:"type"`
	Mode   string        `json:"mode"`
	Fields []schemaField `json:"fields"`
}

// parseSchema reads the JSON that `bq show --schema` prints.
func parseSchema(data []byte) ([]bq.Field, error) {
	var fields []schemaField

	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}

	return bigqueryFields(fields), nil
}

func bigqueryFields(fields []schemaField) []bq.Field {
	converted := make([]bq.Field, len(fields))

	for index, field := range fields {
		converted[index] = bq.Field{
			Name:     field.Name,
			Type:     field.Type,
			Repeated: strings.EqualFold(field.Mode, "REPEATED"),
			Fields:   bigqueryFields(field.Fields),
		}
	}

	return converted
}

// readSchema reads the schema file of a table, at <schemas>/<table>.json.
func (c bigqueryConfig) readSchema(cfg *config, table string) ([]bq.Field, error) {
	file := filepath.Join(cfg.path(c.Schemas), table+".json")

	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s doesn't exist, write it with: bq show --schema --format=prettyjson %s:%s.%s > %s",
			file, c.Project, c.Tables, table, file)
	}

	if err != nil {
		return nil, err
	}

	fields, err := parseSchema(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}

	// A table with no fields would render as a view to drop.
	if len(fields) == 0 {
		return nil, fmt.Errorf("%s: no fields, was it written with bq show --schema?", file)
	}

	return fields, nil
}

// readSchemas reads the schema file of every table in the directory, as the schema that
// [umbra.Catalog.Check] compares the catalog with.
func (c bigqueryConfig) readSchemas(cfg *config) (umbra.Schema, error) {
	directory := cfg.path(c.Schemas)

	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("schemas %q isn't a directory", c.Schemas)
	}

	schema := umbra.Schema{}

	for _, entry := range entries {
		table, isSchemaFile := strings.CutSuffix(entry.Name(), ".json")
		if !isSchemaFile {
			continue
		}

		fields, err := c.readSchema(cfg, table)
		if err != nil {
			return nil, err
		}

		schema[table] = bq.Flatten(fields)
	}

	return schema, nil
}
