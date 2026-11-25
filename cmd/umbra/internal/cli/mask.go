package cli

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/feds01/umbra"
)

// Mask is how a masker's value is computed: exactly one of its keys is set.
type Mask struct {
	Extract string `toml:"extract"`
	Pattern string `toml:"pattern"`
	HMAC    *HMAC  `toml:"hmac"`
}

// HMAC is a keyed pseudonym: the column that keys it, and the SQL that yields the key.
type HMAC struct {
	Scope string `toml:"scope"`
	Key   string `toml:"key"`
}

// extracts are the names that umbra.toml gives the patterns that umbra ships.
var extracts = map[string]string{
	"email": umbra.EmailPattern,
	"url":   umbra.URLPattern,
}

// Expressions turns each mask into a target's expression. hmac is nil for a target that
// has no keyed hash.
func Expressions[Expr any](
	masks map[string]Mask,
	extract func(pattern string) Expr,
	hmac func(scope, key string) Expr,
) (map[string]Expr, error) {
	computed := map[string]Expr{}

	for _, term := range slices.Sorted(maps.Keys(masks)) {
		expression, err := expressionOf(masks[term], extract, hmac)
		if err != nil {
			return nil, fmt.Errorf("mask %s: %w", term, err)
		}

		computed[term] = expression
	}

	return computed, nil
}

func expressionOf[Expr any](
	mask Mask,
	extract func(pattern string) Expr,
	hmac func(scope, key string) Expr,
) (Expr, error) {
	var none Expr

	keys := 0

	for _, isSet := range []bool{mask.Extract != "", mask.Pattern != "", mask.HMAC != nil} {
		if isSet {
			keys++
		}
	}

	if keys != 1 {
		return none, errors.New("want exactly one of extract, pattern or hmac")
	}

	switch {
	case mask.Extract != "":
		pattern, isKnown := extracts[mask.Extract]
		if !isKnown {
			return none, fmt.Errorf("unknown extract %q, want email or url", mask.Extract)
		}

		return extract(pattern), nil
	case mask.Pattern != "":
		return extract(mask.Pattern), nil
	case hmac == nil:
		return none, errors.New("this dialect has no hmac, use extract or pattern")
	case mask.HMAC.Scope == "" || mask.HMAC.Key == "":
		return none, errors.New("hmac needs a scope, the column that keys the pseudonym, and a key, the SQL that yields it")
	}

	return hmac(mask.HMAC.Scope, mask.HMAC.Key), nil
}
