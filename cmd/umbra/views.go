package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/feds01/umbra/cmd/umbra/internal/cli"
)

// views dispatches to the target.
func views(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("views needs a dialect, postgres or bigquery")
	}

	switch dialect, rest := args[0], args[1:]; dialect {
	case "postgres":
		return viewsPostgres(rest, stdout, stderr)
	case "bigquery":
		return viewsBigQuery(rest, stdout, stderr)
	default:
		return fmt.Errorf("unknown dialect %q, want postgres or bigquery", dialect)
	}
}

// viewsPostgres prints the views of the named tables, or of every table of the dataset
// and then the classification view when none is named.
func viewsPostgres(args []string, stdout, stderr io.Writer) error {
	command := cli.NewFlagSet("views postgres [flags] [table...]", stderr)
	replace := command.Bool("replace", false, "drop each view before creating it, for a migration that changes one")

	if err := command.Read(args); err != nil {
		return err
	}

	tables, err := command.Tables()
	if err != nil {
		return err
	}

	cfg, err := readConfig(command.Config)
	if err != nil {
		return err
	}

	section, err := cfg.postgres()
	if err != nil {
		return err
	}

	sql, err := section.SQL(&cfg.Project, *replace, tables)
	if err != nil {
		return err
	}

	fmt.Fprint(stdout, sql)

	return nil
}

// viewsBigQuery prints the view of each named table, or of every table of the dataset.
func viewsBigQuery(args []string, stdout, stderr io.Writer) error {
	command := cli.NewFlagSet("views bigquery [flags] [table...]", stderr)

	if err := command.Read(args); err != nil {
		return err
	}

	tables, err := command.Tables()
	if err != nil {
		return err
	}

	cfg, err := readConfig(command.Config)
	if err != nil {
		return err
	}

	section, err := cfg.bigquery()
	if err != nil {
		return err
	}

	sql, err := section.SQL(&cfg.Project, tables)
	if err != nil {
		return err
	}

	fmt.Fprint(stdout, sql)

	return nil
}
