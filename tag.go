package umbra

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ParseTag parses a tag against the policy's vocabulary:
//
//	tag = "allow" [ "=" basis ] | "deny=" reason | term { "," term }
//
// where each term names one of the policy's maskers. A class prints back, with String,
// as the tag it was parsed from.
func ParseTag(tag string, policy Policy) (Class, error) {
	if tag == "" {
		return nil, errors.New("empty tag")
	}

	if tag == "allow" {
		return Allow{}, nil
	}

	if basis, isAllow := strings.CutPrefix(tag, "allow="); isAllow {
		return parseAllow(Basis(basis), policy)
	}

	if tag == "deny" {
		return nil, fmt.Errorf("deny needs a reason: %s", oneOf(policy.Reasons))
	}

	if reason, isDeny := strings.CutPrefix(tag, "deny="); isDeny {
		return parseDeny(Reason(reason), policy)
	}

	return parseMask(tag, policy)
}

func parseAllow(basis Basis, policy Policy) (Class, error) {
	switch {
	case slices.Contains(policy.Bases, basis):
		return Allow{Basis: basis}, nil
	case len(policy.Bases) == 0:
		return nil, fmt.Errorf("unknown basis %q; the policy has none", basis)
	}

	return nil, fmt.Errorf("unknown basis %q; use %s", basis, oneOf(policy.Bases))
}

func parseDeny(reason Reason, policy Policy) (Class, error) {
	switch {
	case slices.Contains(policy.Reasons, reason):
		return Deny{Reason: reason}, nil
	case reason == "":
		return nil, fmt.Errorf("deny needs a reason: %s", oneOf(policy.Reasons))
	}

	return nil, fmt.Errorf("unknown reason %q; use %s", reason, oneOf(policy.Reasons))
}

func parseMask(tag string, policy Policy) (Class, error) {
	var mask Mask

	for term := range strings.SplitSeq(tag, ",") {
		masker, err := policy.masker(term)
		if err != nil {
			return nil, err
		}

		if slices.ContainsFunc(mask.Maskers, func(seen Masker) bool { return seen.Term == term }) {
			return nil, fmt.Errorf("%s appears twice", term)
		}

		mask.Maskers = append(mask.Maskers, masker)
	}

	return mask, nil
}

// masker finds the masker a term names.
func (p Policy) masker(term string) (Masker, error) {
	if index := slices.IndexFunc(p.Maskers, func(masker Masker) bool { return masker.Term == term }); index >= 0 {
		return p.Maskers[index], nil
	}

	switch {
	case term == "":
		return Masker{}, errors.New("empty term")
	case isAllowOrDeny(term):
		return Masker{}, fmt.Errorf("%s stands alone; it can't be combined with other terms", term)
	}

	choices := []string{"allow", "deny=<reason>"}
	for _, masker := range p.Maskers {
		choices = append(choices, masker.Term)
	}

	return Masker{}, fmt.Errorf("unknown term %q; use %s", term, oneOf(choices))
}
