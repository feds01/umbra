package cli

import (
	"testing"

	"github.com/feds01/umbra"
)

func TestAMaskerReadsEveryKindItNames(t *testing.T) {
	policy := PolicyConfig{Maskers: []MaskerConfig{{Term: "pseudonymise", Suffix: "pseudonym", Kinds: []string{"text", "uuid"}}}}

	got, err := policy.policy()
	if err != nil {
		t.Fatal(err)
	}

	if want := umbra.Text | umbra.UUID; got.Maskers[0].Kinds != want {
		t.Errorf("got kinds %v, want %v", got.Maskers[0].Kinds, want)
	}
}
