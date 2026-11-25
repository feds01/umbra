// Package cli is what the commands of umbra share: the part of umbra.toml that every
// target reads, the catalog it names, the flags, and how a failure is told from a
// problem with the catalog. It knows no target.
package cli

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing/fstest"

	"github.com/BurntSushi/toml"
	"github.com/feds01/umbra"
)

// ConfigName is the file that holds a project's policy and the views to render.
const ConfigName = "umbra.toml"

// Project is the part of umbra.toml that is not a target's section: the catalog and the
// policy its tags are written in. The paths in it are relative to the file.
type Project struct {
	Catalog string       `toml:"catalog"`
	Policy  PolicyConfig `toml:"policy"`

	dir string
}

// PolicyConfig is the [policy] table, which is umbra.Policy but for its rules, as they
// are Go functions.
type PolicyConfig struct {
	Reasons []string       `toml:"reasons"`
	Bases   []string       `toml:"bases"`
	Maskers []MaskerConfig `toml:"maskers"`
}

// MaskerConfig is one of the [[policy.maskers]].
type MaskerConfig struct {
	Term   string   `toml:"term"`
	Suffix string   `toml:"suffix"`
	Kinds  []string `toml:"kinds"`
	Reason string   `toml:"reason"`
	Reads  []string `toml:"reads"`
}

// kinds are the names that umbra.toml gives the kinds of value a masker reads.
var kinds = map[string]umbra.Kind{
	"text":   umbra.Text,
	"uuid":   umbra.UUID,
	"inet":   umbra.Inet,
	"scalar": umbra.Scalar,
}

// Read decodes umbra.toml, or the nearest one when path is empty, into file, which embeds
// project and adds the sections of the targets. A key that nothing reads is an error.
func Read(path string, file any, project *Project) error {
	if path == "" {
		found, err := find()
		if err != nil {
			return err
		}

		path = found
	}

	meta, err := toml.DecodeFile(path, file)
	if err != nil {
		return err
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		names := make([]string, len(undecoded))
		for index, name := range undecoded {
			names[index] = name.String()
		}

		return fmt.Errorf("%s: unknown %s", path, strings.Join(names, ", "))
	}

	if project.Catalog == "" {
		return fmt.Errorf("%s: catalog names no directory", path)
	}

	project.dir = filepath.Dir(path)

	return nil
}

// find returns the path of the umbra.toml nearest to the working directory, at or above it.
func find() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, ConfigName)

		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s in the working directory or above it, name one with --config", ConfigName)
		}

		dir = parent
	}
}

// Path resolves a path in umbra.toml.
func (p *Project) Path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}

	return filepath.Join(p.dir, name)
}

// LoadCatalog loads the catalog files against the policy. A policy or a directory that
// can't be used is an error, and a table the policy refuses is a problem with the catalog.
func (p *Project) LoadCatalog() (*umbra.Catalog, error) {
	policy, err := p.Policy.policy()
	if err != nil {
		return nil, err
	}

	// An empty catalog loads only if the policy itself is sound.
	if _, err := umbra.Load(fstest.MapFS{}, policy); err != nil {
		return nil, err
	}

	directory := p.Path(p.Catalog)

	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("catalog %q isn't a directory", p.Catalog)
	}

	catalog, err := umbra.Load(os.DirFS(directory), policy)
	if err != nil {
		return nil, NewProblem(err)
	}

	return catalog, nil
}

func (c PolicyConfig) policy() (umbra.Policy, error) {
	var policy umbra.Policy

	for _, reason := range c.Reasons {
		policy.Reasons = append(policy.Reasons, umbra.Reason(reason))
	}

	for _, basis := range c.Bases {
		policy.Bases = append(policy.Bases, umbra.Basis(basis))
	}

	for _, masker := range c.Maskers {
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

// CheckMasks reports a mask for a term the policy doesn't declare, which would only ever
// leave a column hidden.
func (p *Project) CheckMasks(masks map[string]Mask) error {
	declared := map[string]bool{}
	for _, masker := range p.Policy.Maskers {
		declared[masker.Term] = true
	}

	for _, term := range slices.Sorted(maps.Keys(masks)) {
		if !declared[term] {
			return fmt.Errorf("mask %s: the policy declares no masker with this term", term)
		}
	}

	return nil
}
