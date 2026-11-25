// Package postgres is the umbra command's PostgreSQL target: its section of umbra.toml,
// the views it renders and the database it checks. It uses the library's postgres package
// and nothing of any other target.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/feds01/umbra"
	pg "github.com/feds01/umbra/postgres"
	_ "github.com/lib/pq" // the driver of sql.Open("postgres", ...)

	"github.com/feds01/umbra/cmd/umbra/internal/cli"
)

// Config is the [postgres] section of umbra.toml.
type Config struct {
	Dataset        string              `toml:"dataset"`
	Tables         string              `toml:"tables"`
	Views          string              `toml:"views"`
	Role           string              `toml:"role"`
	Classification string              `toml:"classification"`
	Masks          map[string]cli.Mask `toml:"masks"`
}

// filled returns the section with its defaults, or says what it lacks.
func (c Config) filled() (Config, error) {
	if c.Dataset == "" || c.Views == "" {
		return Config{}, errors.New("[postgres] needs a dataset and views")
	}

	if c.Tables == "" {
		c.Tables = "public"
	}

	return c, nil
}

// SQL returns the SQL of the views of the named tables, or of every table of the dataset
// and then the classification view when none is named. It returns nothing unless it can
// render all of it.
func (c Config) SQL(project *cli.Project, replace bool, tables []string) (string, error) {
	section, err := c.filled()
	if err != nil {
		return "", err
	}

	if err := project.CheckMasks(section.Masks); err != nil {
		return "", err
	}

	masks, err := cli.Expressions(section.Masks, pg.Extract, pg.HMAC)
	if err != nil {
		return "", err
	}

	catalog, err := project.LoadCatalog()
	if err != nil {
		return "", err
	}

	renderer := pg.Views{Schema: section.Views, Source: section.Tables, Role: section.Role, Masks: masks, Replace: replace}

	var out strings.Builder

	views, err := renderer.SQL(catalog, section.Dataset, tables...)
	if err != nil {
		return "", cli.NewProblem(err)
	}

	out.WriteString(views)

	if len(tables) == 0 && section.Classification != "" {
		classification, err := renderer.ClassificationSQL(catalog, section.Dataset, section.Classification)
		if err != nil {
			return "", cli.NewProblem(err)
		}

		out.WriteString("\n" + classification)
	}

	return out.String(), nil
}

// Check compares the dataset in the catalog with the schema of the database at the address.
func (c Config) Check(catalog *umbra.Catalog, address string) error {
	section, err := c.filled()
	if err != nil {
		return err
	}

	schema, err := readSchema(address, section.Tables)
	if err != nil {
		return err
	}

	if err := catalog.Check(section.Dataset, schema); err != nil {
		return cli.NewProblem(err)
	}

	return nil
}

// readSchema reads the columns of the database at the address, naming the tables of the
// schema bare.
func readSchema(address, schema string) (umbra.Schema, error) {
	db, err := sql.Open("postgres", address)
	if err != nil {
		return nil, withoutSecrets(err, address)
	}

	defer db.Close()

	tables, err := pg.ReadSchema(context.Background(), db, schema)
	if err != nil {
		return nil, withoutSecrets(err, address)
	}

	return tables, nil
}

// passwordSetting matches a password in a connection string of keywords.
var passwordSetting = regexp.MustCompile(`(?i)password\s*=\s*('[^']*'|\S+)`)

// withoutSecrets hides the database's address, and any password in it, from an error: a
// driver quotes what it couldn't parse, and a terminal would show it.
func withoutSecrets(err error, address string) error {
	message := strings.ReplaceAll(err.Error(), address, "<database>")

	if parsed, parseErr := url.Parse(address); parseErr == nil {
		if password, isSet := parsed.User.Password(); isSet && password != "" {
			message = strings.ReplaceAll(message, password, "***")
		}
	}

	return errors.New(passwordSetting.ReplaceAllString(message, "password=***"))
}
