// Package bigquery is the umbra command's BigQuery target: its section of umbra.toml, the
// views it renders and the schema files it checks. It uses the library's bigquery package
// and nothing of any other target.
package bigquery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/feds01/umbra"
	bq "github.com/feds01/umbra/bigquery"

	"github.com/feds01/umbra/cmd/umbra/internal/cli"
)

// Config is the [bigquery] section of umbra.toml.
type Config struct {
	Dataset string              `toml:"dataset"`
	Project string              `toml:"project"`
	Tables  string              `toml:"tables"`
	Views   string              `toml:"views"`
	Schemas string              `toml:"schemas"`
	Masks   map[string]cli.Mask `toml:"masks"`
}

// validate says what the section lacks.
func (c Config) validate() error {
	if c.Dataset == "" || c.Project == "" || c.Tables == "" || c.Views == "" || c.Schemas == "" {
		return errors.New("[bigquery] needs a dataset, project, tables, views and schemas")
	}

	return nil
}

// SQL returns the SQL of the view of each named table, or of every table of the dataset,
// over the schema file that `bq show --schema` wrote for it. It returns nothing unless it
// can render all of them.
func (c Config) SQL(project *cli.Project, tables []string) (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}

	if err := project.CheckMasks(c.Masks); err != nil {
		return "", err
	}

	masks, err := cli.Expressions(c.Masks, bq.Extract, nil)
	if err != nil {
		return "", err
	}

	catalog, err := project.LoadCatalog()
	if err != nil {
		return "", err
	}

	if len(tables) == 0 {
		for table := range catalog.Tables(c.Dataset) {
			tables = append(tables, table.Name)
		}
	}

	renderer := bq.Views{Project: c.Project, Dataset: c.Views, Masks: masks}

	rendered := make([]string, len(tables))

	for index, name := range tables {
		table, isFound := catalog.Table(c.Dataset, name)
		if !isFound {
			return "", cli.NewProblem(fmt.Errorf("%s %s: not in the catalog", c.Dataset, name))
		}

		fields, err := c.readSchema(project, name)
		if err != nil {
			return "", err
		}

		view, err := renderer.View(table, bq.Source{Dataset: c.Tables, Table: name}, fields)
		if err != nil {
			return "", cli.NewProblem(err)
		}

		rendered[index] = view.SQL + ";\n"
	}

	return strings.Join(rendered, "\n"), nil
}

// Check compares the dataset in the catalog with the schema files.
func (c Config) Check(project *cli.Project, catalog *umbra.Catalog) error {
	if err := c.validate(); err != nil {
		return err
	}

	schema, err := c.readSchemas(project)
	if err != nil {
		return err
	}

	if err := catalog.Check(c.Dataset, schema); err != nil {
		return cli.NewProblem(err)
	}

	return nil
}

// readSchema reads the schema file of a table, at <schemas>/<table>.json.
func (c Config) readSchema(project *cli.Project, table string) ([]bq.Field, error) {
	file := filepath.Join(project.Path(c.Schemas), table+".json")

	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s doesn't exist, write it with: bq show --schema --format=prettyjson %s:%s.%s > %s",
			file, c.Project, c.Tables, table, file)
	}

	if err != nil {
		return nil, err
	}

	fields, err := bq.ParseSchema(data)
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
func (c Config) readSchemas(project *cli.Project) (umbra.Schema, error) {
	entries, err := os.ReadDir(project.Path(c.Schemas))
	if err != nil {
		return nil, fmt.Errorf("schemas %q isn't a directory", c.Schemas)
	}

	schema := umbra.Schema{}

	for _, entry := range entries {
		table, isSchemaFile := strings.CutSuffix(entry.Name(), ".json")
		if !isSchemaFile {
			continue
		}

		fields, err := c.readSchema(project, table)
		if err != nil {
			return nil, err
		}

		schema[table] = bq.Flatten(fields)
	}

	return schema, nil
}
