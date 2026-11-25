package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/feds01/umbra"
	"github.com/feds01/umbra/postgres"
	_ "github.com/lib/pq"
)

// check loads the catalog, and compares it with the schema of the PostgreSQL database
// that --postgres names.
func check(args []string, stdout, stderr io.Writer) error {
	command := newFlagSet("check [flags]", stderr)
	address := command.String("postgres", "", "also compare the [postgres] dataset with the schema of the database at this URL")

	if err := command.parse(args); err != nil {
		return err
	}

	// A URL that was meant for --postgres must not make the check pass without a database.
	if err := command.none(); err != nil {
		return err
	}

	// 1. Load the catalog files against the policy.
	cfg, err := readConfig(command.config)
	if err != nil {
		return err
	}

	catalog, err := cfg.catalog()
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "the catalog is valid")

	if *address == "" {
		return nil
	}

	// 2. Compare the PostgreSQL dataset with the database's schema.
	section, err := cfg.postgres()
	if err != nil {
		return err
	}

	schema, err := readPostgresSchema(*address, section.Tables)
	if err != nil {
		return err
	}

	if err := catalog.Check(section.Dataset, schema); err != nil {
		return problem{err}
	}

	fmt.Fprintf(stdout, "%s matches the database\n", section.Dataset)

	return nil
}

// readPostgresSchema reads the columns of the database at the address, naming the tables
// of the schema bare.
func readPostgresSchema(address, schema string) (umbra.Schema, error) {
	db, err := sql.Open("postgres", address)
	if err != nil {
		return nil, withoutSecrets(err, address)
	}

	defer db.Close()

	tables, err := postgres.ReadSchema(context.Background(), db, schema)
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
