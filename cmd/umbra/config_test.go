package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/feds01/umbra"
)

// maskerA is a policy that declares the masker "a", for the tests of its mask.
const maskerA = "[policy]\nreasons = [\"person\"]\n[[policy.maskers]]\nterm = \"a\"\nsuffix = \"a\"\nkinds = [\"text\"]\n"

// postgresSection is a [postgres] section that needs only the masks and maskers a test adds.
const postgresSection = "[postgres]\ndataset = \"d\"\nviews = \"v\"\n"

func TestAConfigThatCantBeUsedExitsWithTwo(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		config string
		want   string
	}{
		{"a missing config", []string{"views", "postgres", "--config", "missing.toml"}, "", "missing.toml"},
		{"an unknown key", nil, "catalog = \"catalog\"\ncatlog = 1\n", "unknown catlog"},
		{"no catalog", nil, "[policy]\n", "catalog names no directory"},
		{"a catalog that isn't there", nil, "catalog = \"catlog\"\n[policy]\nreasons = [\"person\"]\n" + postgresSection, `catalog "catlog" isn't a directory`},
		{"a policy with no reason to deny with", nil, "catalog = \".\"\n" + postgresSection, "no reason to deny"},
		{"an unknown kind", nil, "catalog = \".\"\n" + postgresSection + "[[policy.maskers]]\nterm = \"a\"\nkinds = [\"blob\"]\n", `unknown kind "blob"`},
		{"no postgres section", nil, "catalog = \".\"\n", "has no [postgres] section"},
		{"a mask with two keys", nil, "catalog = \".\"\n" + maskerA + postgresSection + "masks.a = { extract = \"email\", pattern = \"x\" }\n", "mask a: want exactly one of extract, pattern or hmac"},
		{"an hmac with an extract", nil, "catalog = \".\"\n" + maskerA + postgresSection + "masks.a = { extract = \"email\", hmac = { scope = \"s\", key = \"k\" } }\n", "mask a: want exactly one of"},
		{"a mask with no key", nil, "catalog = \".\"\n" + maskerA + postgresSection + "masks.a = {}\n", "mask a: want exactly one of"},
		{"an hmac with nothing in it", nil, "catalog = \".\"\n" + maskerA + postgresSection + "masks.a = { hmac = {} }\n", "hmac needs a scope"},
		{"an hmac with no key", nil, "catalog = \".\"\n" + maskerA + postgresSection + "masks.a = { hmac = { scope = \"org_id\" } }\n", "hmac needs a scope"},
		{"an unknown extract", nil, "catalog = \".\"\n" + maskerA + postgresSection + "masks.a = { extract = \"phone\" }\n", `unknown extract "phone"`},
		{"a mask for a term the policy lacks", nil, "catalog = \".\"\n[policy]\nreasons = [\"person\"]\n" + postgresSection + "masks.typo = { extract = \"email\" }\n", "mask typo: the policy declares no masker with this term"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := test.args

			if test.config != "" {
				args = []string{"views", "postgres", "--config", project(t, map[string]string{configName: test.config})}
			}

			code, stdout, stderr := execute(t, args...)

			if code != 2 || stdout != "" || !strings.Contains(stderr, test.want) {
				t.Errorf("got exit code %d, %q and %q, want 2, nothing and %q", code, stdout, stderr, test.want)
			}
		})
	}
}

func TestEveryBadCatalogFileIsReportedWithExitCodeOne(t *testing.T) {
	config := project(t, map[string]string{
		configName:                      "catalog = \"catalog\"\n[policy]\nreasons = [\"person\"]\n" + postgresSection,
		"catalog/postgres/contacts.pii": "id allow\nname deny=nobody\nemail\n",
	})

	code, stdout, stderr := execute(t, "views", "postgres", "--config", config)

	if code != 1 || stdout != "" {
		t.Fatalf("got exit code %d and %q, want 1 and nothing: %s", code, stdout, stderr)
	}

	for _, want := range []string{"contacts.pii:2", "contacts.pii:3: want a column and its tag"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in %q", want, stderr)
		}
	}
}

func TestACatalogCanBeNamedByAnAbsolutePath(t *testing.T) {
	catalog := filepath.Dir(project(t, map[string]string{"postgres/contacts.pii": "id allow\n"}))

	config := project(t, map[string]string{
		configName: "catalog = " + `"` + filepath.ToSlash(catalog) + `"` + "\n[policy]\nreasons = [\"person\"]\n[postgres]\ndataset = \"postgres\"\nviews = \"v\"\n",
	})

	code, stdout, stderr := execute(t, "views", "postgres", "--config", config)

	if code != 0 || !strings.Contains(stdout, `CREATE VIEW v."contacts"`) {
		t.Errorf("got exit code %d, %q and %q", code, stdout, stderr)
	}
}

func TestAMaskerReadsEveryKindItNames(t *testing.T) {
	var policy policyConfig

	policy.Maskers = append(policy.Maskers, struct {
		Term   string   `toml:"term"`
		Suffix string   `toml:"suffix"`
		Kinds  []string `toml:"kinds"`
		Reason string   `toml:"reason"`
		Reads  []string `toml:"reads"`
	}{Term: "pseudonymise", Suffix: "pseudonym", Kinds: []string{"text", "uuid"}})

	got, err := policy.policy()
	if err != nil {
		t.Fatal(err)
	}

	if want := umbra.Text | umbra.UUID; got.Maskers[0].Kinds != want {
		t.Errorf("got kinds %v, want %v", got.Maskers[0].Kinds, want)
	}
}
