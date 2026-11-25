package main

import (
	"fmt"

	"github.com/feds01/umbra/cmd/umbra/internal/bigquery"
	"github.com/feds01/umbra/cmd/umbra/internal/cli"
	"github.com/feds01/umbra/cmd/umbra/internal/postgres"
)

// config is umbra.toml: the project that every target reads, and a section for each target.
type config struct {
	cli.Project

	Postgres *postgres.Config `toml:"postgres"`
	BigQuery *bigquery.Config `toml:"bigquery"`
}

// readConfig reads umbra.toml, or the nearest one when path is empty.
func readConfig(path string) (*config, error) {
	var cfg config

	if err := cli.Read(path, &cfg, &cfg.Project); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// postgres returns the [postgres] section, or says there is none.
func (c *config) postgres() (postgres.Config, error) {
	if c.Postgres == nil {
		return postgres.Config{}, fmt.Errorf("%s has no [postgres] section", cli.ConfigName)
	}

	return *c.Postgres, nil
}

// bigquery returns the [bigquery] section, or says there is none.
func (c *config) bigquery() (bigquery.Config, error) {
	if c.BigQuery == nil {
		return bigquery.Config{}, fmt.Errorf("%s has no [bigquery] section", cli.ConfigName)
	}

	return *c.BigQuery, nil
}
