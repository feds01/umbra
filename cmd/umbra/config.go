package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing/fstest"

	"github.com/BurntSushi/toml"
	"github.com/feds01/umbra"
	"github.com/feds01/umbra/postgres"
)

// configName is the file that holds a project's policy and the views to render.
const configName = "umbra.toml"

// config is umbra.toml. The paths in it are relative to the file.
type config struct {
	Catalog  string          `toml:"catalog"`
	Policy   policyConfig    `toml:"policy"`
	Postgres *postgresConfig `toml:"postgres"`

	dir string
}

type policyConfig struct {
	Reasons []string `toml:"reasons"`
	Bases   []string `toml:"bases"`
	Maskers []struct {
		Term   string   `toml:"term"`
		Suffix string   `toml:"suffix"`
		Kinds  []string `toml:"kinds"`
		Reason string   `toml:"reason"`
		Reads  []string `toml:"reads"`
	} `toml:"maskers"`
}

type postgresConfig struct {
	Dataset        string                `toml:"dataset"`
	Tables         string                `toml:"tables"`
	Views          string                `toml:"views"`
	Role           string                `toml:"role"`
	Classification string                `toml:"classification"`
	Masks          map[string]maskConfig `toml:"masks"`
}

// kinds are the names that umbra.toml gives the kinds of value a masker reads.
var kinds = map[string]umbra.Kind{
	"text":   umbra.Text,
	"uuid":   umbra.UUID,
	"inet":   umbra.Inet,
	"scalar": umbra.Scalar,
}

// findConfig returns the path of the umbra.toml nearest to the working directory, at
// or above it.
func findConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, configName)

		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s in the working directory or above it, name one with --config", configName)
		}

		dir = parent
	}
}

// readConfig reads umbra.toml, or the nearest one when path is empty.
func readConfig(path string) (*config, error) {
	if path == "" {
		found, err := findConfig()
		if err != nil {
			return nil, err
		}

		path = found
	}

	var cfg config

	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, err
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		names := make([]string, len(undecoded))
		for index, name := range undecoded {
			names[index] = name.String()
		}

		return nil, fmt.Errorf("%s: unknown %s", path, strings.Join(names, ", "))
	}

	if cfg.Catalog == "" {
		return nil, fmt.Errorf("%s: catalog names no directory", path)
	}

	cfg.dir = filepath.Dir(path)

	return &cfg, nil
}

// path resolves a path in umbra.toml.
func (c *config) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}

	return filepath.Join(c.dir, name)
}

// catalog loads the catalog files against the policy. A policy or a directory that can't be
// used is an error, and a table the policy refuses is a problem with the catalog.
func (c *config) catalog() (*umbra.Catalog, error) {
	policy, err := c.Policy.policy()
	if err != nil {
		return nil, err
	}

	// An empty catalog loads only if the policy itself is sound.
	if _, err := umbra.Load(fstest.MapFS{}, policy); err != nil {
		return nil, err
	}

	directory := c.path(c.Catalog)

	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("catalog %q isn't a directory", c.Catalog)
	}

	catalog, err := umbra.Load(os.DirFS(directory), policy)
	if err != nil {
		return nil, problem{err}
	}

	return catalog, nil
}

func (p policyConfig) policy() (umbra.Policy, error) {
	var policy umbra.Policy

	for _, reason := range p.Reasons {
		policy.Reasons = append(policy.Reasons, umbra.Reason(reason))
	}

	for _, basis := range p.Bases {
		policy.Bases = append(policy.Bases, umbra.Basis(basis))
	}

	for _, masker := range p.Maskers {
		var kind umbra.Kind

		for _, name := range masker.Kinds {
			bit, isKind := kinds[name]
			if !isKind {
				return umbra.Policy{}, fmt.Errorf("masker %s: unknown kind %q, want text, uuid, inet or scalar", masker.Term, name)
			}

			kind |= bit
		}

		policy.Maskers = append(policy.Maskers, umbra.Masker{
			Term:   masker.Term,
			Suffix: masker.Suffix,
			Kinds:  kind,
			Reason: umbra.Reason(masker.Reason),
			Reads:  masker.Reads,
		})
	}

	return policy, nil
}

// postgres returns the [postgres] section, or says there is none.
func (c *config) postgres() (postgresConfig, error) {
	section := c.Postgres

	switch {
	case section == nil:
		return postgresConfig{}, fmt.Errorf("%s has no [postgres] section", configName)
	case section.Dataset == "" || section.Views == "":
		return postgresConfig{}, errors.New("[postgres] needs a dataset and views")
	}

	filled := *section

	if filled.Tables == "" {
		filled.Tables = "public"
	}

	return filled, nil
}

// checkMasks reports a mask for a term the policy doesn't declare, which would only ever
// leave a column hidden.
func (c *config) checkMasks(masks map[string]maskConfig) error {
	declared := map[string]bool{}
	for _, masker := range c.Policy.Maskers {
		declared[masker.Term] = true
	}

	for _, term := range slices.Sorted(maps.Keys(masks)) {
		if !declared[term] {
			return fmt.Errorf("mask %s: the policy declares no masker with this term", term)
		}
	}

	return nil
}

// masks turns each mask into its PostgreSQL expression.
func (c postgresConfig) masks(cfg *config) (map[string]postgres.Expr, error) {
	if err := cfg.checkMasks(c.Masks); err != nil {
		return nil, err
	}

	return expressions(c.Masks, postgres.Extract, postgres.HMAC)
}
