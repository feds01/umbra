package umbra

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// A Policy is the vocabulary a tag may use: the reasons to deny a column, the bases to
// allow one that can hold anything, and the maskers that may show a derived value instead.
type Policy struct {
	Reasons []Reason
	Bases   []Basis
	Maskers []Masker
	// Rules are the policy's own checks, run on each table of a catalog.
	Rules []Rule
}

// A Masker derives a value a view may show in place of a column's.
type Masker struct {
	// Term names the masker in a tag, such as "generalise=domain".
	Term string
	// Suffix names the view column it shows: email becomes email_domain.
	Suffix string
	// Kinds are the kinds of value it can read.
	Kinds Kind
	// Reason is why the column is masked rather than allowed. It is optional.
	Reason Reason
	// Reads lists other columns it reads, which each table it masks must allow.
	Reads []string
}

// A Rule checks one table of a catalog against a constraint of the policy's own.
type Rule func(dataset string, table Table) error

// A Kind is a set of kinds of value. Its zero value, [Opaque], holds no kind a masker can read.
type Kind uint8

// Opaque is a value that can hold anything, such as JSON or bytes. No masker reads one,
// and only an allow with a basis shows one.
const Opaque Kind = 0

// The kinds of value a schema reader recognises.
const (
	Text Kind = 1 << iota
	UUID
	Inet
	// Scalar is a number, boolean, time or enum.
	Scalar
)

// String names the kinds in the set, such as "text, uuid or inet".
func (k Kind) String() string {
	if k == Opaque {
		return "opaque"
	}

	var names []string

	for bit, name := range []string{"text", "uuid", "inet", "scalar"} {
		if k&(1<<bit) != 0 {
			names = append(names, name)
		}
	}

	return oneOf(names)
}

// check reports what makes the policy unusable: no reason to deny with, a word a tag
// can't hold, a word declared twice, or a masker that can't be told apart from allow or
// deny or that reads nothing.
func (p Policy) check() error {
	terms := make([]string, len(p.Maskers))
	suffixes := make([]string, len(p.Maskers))

	for index, masker := range p.Maskers {
		terms[index] = masker.Term
		suffixes[index] = masker.Suffix
	}

	var problems []error

	if len(p.Reasons) == 0 {
		problems = append(problems, errors.New("policy: no reason to deny a column with"))
	}

	problems = append(problems, checkWords("reason", p.Reasons)...)
	problems = append(problems, checkWords("basis", p.Bases)...)
	problems = append(problems, checkWords("term", terms)...)
	problems = append(problems, checkWords("suffix", suffixes)...)

	for _, masker := range p.Maskers {
		if isAllowOrDeny(masker.Term) {
			problems = append(problems, fmt.Errorf("policy: term %q reads as allow or deny", masker.Term))
		}

		if masker.Kinds == Opaque {
			problems = append(problems, fmt.Errorf("policy: %s reads no kind of value", masker.Term))
		}
	}

	return errors.Join(problems...)
}

// checkWords reports each word that is empty, repeated, or holds a space or a comma.
func checkWords[Word ~string](what string, words []Word) []error {
	var problems []error

	seen := map[Word]bool{}

	for _, word := range words {
		switch {
		case word == "" || strings.ContainsFunc(string(word), isSeparator):
			problems = append(problems, fmt.Errorf("policy: %s %q must be one word", what, word))
		case seen[word]:
			problems = append(problems, fmt.Errorf("policy: %s %q appears twice", what, word))
		}

		seen[word] = true
	}

	return problems
}

func isSeparator(r rune) bool {
	return r == ',' || unicode.IsSpace(r)
}

func isAllowOrDeny(term string) bool {
	return term == "allow" || term == "deny" || strings.HasPrefix(term, "allow=") || strings.HasPrefix(term, "deny=")
}

// oneOf lists words to choose from: "a, b or c".
func oneOf[Word ~string](words []Word) string {
	list := make([]string, len(words))
	for index, word := range words {
		list[index] = string(word)
	}

	if len(list) < 2 {
		return strings.Join(list, "")
	}

	last := len(list) - 1

	return strings.Join(list[:last], ", ") + " or " + list[last]
}
