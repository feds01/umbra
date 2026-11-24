package umbra

import "strings"

// A Class says how much of a column a view may show. It is an [Allow], a [Deny] or a [Mask].
type Class interface {
	// String returns the class as a tag, such as "deny=person".
	String() string
	class()
}

// Allow shows a column's value as it is.
type Allow struct {
	// Basis is why a column that can hold anything, such as JSON, is allowed anyway.
	// Most columns need none.
	Basis Basis
}

// Deny shows nothing of a column.
type Deny struct {
	Reason Reason
}

// Mask shows what each of its maskers derives from a column, in place of its value.
type Mask struct {
	Maskers []Masker
}

// A Reason says why a column is denied or masked, such as "person" or "credential".
type Reason string

// A Basis says why a column that can hold anything is allowed, such as "business_data".
type Basis string

func (Allow) class() {}
func (Deny) class()  {}
func (Mask) class()  {}

func (a Allow) String() string {
	if a.Basis == "" {
		return "allow"
	}

	return "allow=" + string(a.Basis)
}

func (d Deny) String() string {
	return "deny=" + string(d.Reason)
}

func (m Mask) String() string {
	terms := make([]string, len(m.Maskers))
	for index, masker := range m.Maskers {
		terms[index] = masker.Term
	}

	return strings.Join(terms, ",")
}
