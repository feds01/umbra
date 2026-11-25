package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/feds01/umbra/postgres"
)

// views dispatches to the dialect.
func views(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("views needs a dialect, postgres")
	}

	switch dialect, rest := args[0], args[1:]; dialect {
	case "postgres":
		return viewsPostgres(rest, stdout, stderr)
	default:
		return fmt.Errorf("unknown dialect %q, want postgres", dialect)
	}
}

// viewsPostgres prints the views of the named tables, or of every table of the dataset
// and then the classification view when none is named. It prints nothing unless it can
// render all of it.
func viewsPostgres(args []string, stdout, stderr io.Writer) error {
	command := newFlagSet("views postgres [flags] [table...]", stderr)
	replace := command.Bool("replace", false, "drop each view before creating it, for a migration that changes one")

	if err := command.parse(args); err != nil {
		return err
	}

	tables, err := command.names()
	if err != nil {
		return err
	}

	cfg, err := readConfig(command.config)
	if err != nil {
		return err
	}

	section, err := cfg.postgres()
	if err != nil {
		return err
	}

	masks, err := section.masks(cfg)
	if err != nil {
		return err
	}

	catalog, err := cfg.catalog()
	if err != nil {
		return err
	}

	renderer := postgres.Views{Schema: section.Views, Source: section.Tables, Role: section.Role, Masks: masks, Replace: *replace}

	var out strings.Builder

	sql, err := renderer.SQL(catalog, section.Dataset, tables...)
	if err != nil {
		return problem{err}
	}

	out.WriteString(sql)

	if len(tables) == 0 && section.Classification != "" {
		classification, err := renderer.ClassificationSQL(catalog, section.Dataset, section.Classification)
		if err != nil {
			return problem{err}
		}

		out.WriteString("\n" + classification)
	}

	fmt.Fprint(stdout, out.String())

	return nil
}
