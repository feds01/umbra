package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/feds01/umbra/cmd/umbra/internal/cli"
)

// check loads the catalog, and compares it with the schema of the PostgreSQL database that
// --postgres names, and with the BigQuery schema files that umbra.toml lists.
func check(args []string, stdout, stderr io.Writer) error {
	command := cli.NewFlagSet("check [flags]", stderr)
	address := command.String("postgres", "", "also compare the [postgres] dataset with the schema of the database at this URL")

	if err := command.Read(args); err != nil {
		return err
	}

	// A URL that was meant for --postgres must not make the check pass without a database.
	if err := command.NoArguments(); err != nil {
		return err
	}

	// 1. Load the catalog files against the policy.
	cfg, err := readConfig(command.Config)
	if err != nil {
		return err
	}

	catalog, err := cfg.LoadCatalog()
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "the catalog is valid")

	var problems []error

	// 2. Compare the PostgreSQL dataset with the database's schema.
	if *address != "" {
		section, err := cfg.postgres()
		if err != nil {
			return err
		}

		if err := section.Check(catalog, *address); err != nil {
			if !isProblem(err) {
				return err
			}

			problems = append(problems, err)
		} else {
			fmt.Fprintf(stdout, "%s matches the database\n", section.Dataset)
		}
	}

	// 3. Compare the BigQuery dataset with the schema files.
	if cfg.BigQuery != nil {
		section, err := cfg.bigquery()
		if err != nil {
			return err
		}

		if err := section.Check(&cfg.Project, catalog); err != nil {
			if !isProblem(err) {
				return err
			}

			problems = append(problems, err)
		} else {
			fmt.Fprintf(stdout, "%s matches the schema files\n", section.Dataset)
		}
	}

	if len(problems) > 0 {
		return cli.NewProblem(errors.Join(problems...))
	}

	return nil
}

// isProblem tells a mismatch with the catalog, which is reported with the others, from a
// check that couldn't run.
func isProblem(err error) bool {
	var found cli.Problem

	return errors.As(err, &found)
}
