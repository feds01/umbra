package cli

import "errors"

// Problem is something wrong with the catalog, as opposed to a command that couldn't run.
type Problem struct{ error }

// NewProblem marks an error as a problem with the catalog.
func NewProblem(err error) Problem { return Problem{err} }

// Unwrap returns the error that was marked.
func (p Problem) Unwrap() error { return p.error }

// ErrUsage marks a command line that was already reported.
var ErrUsage = errors.New("usage")
