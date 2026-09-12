// Package rule names declscope's checks.
//
// The same names are used everywhere a check has to be referred to: the
// category of a diagnostic, the key of a baseline entry, and the target of an
// ignore directive. One vocabulary means that whatever a diagnostic calls
// itself is exactly what can be written to silence or record it.
package rule

import "slices"

// Rule is the name of a check.
type Rule string

const (
	// Boundary: a declaration used from outside the namespace it is private
	// to — a boundary crossing.
	Boundary Rule = "boundary"
	// Qualify: an unexported package-level declaration missing its namespace
	// label, which namespace.Qualify adds.
	Qualify Rule = "qualify"
	// Unqualify: a namespace label present where it is not required, which
	// namespace.Unqualify drops.
	Unqualify Rule = "unqualify"
)

// All lists every rule, in the order they are reported.
var All = []Rule{Boundary, Qualify, Unqualify}

// Parse resolves a rule name.
func Parse(name string) (Rule, bool) {
	r := Rule(name)
	return r, slices.Contains(All, r)
}

// Names returns every rule name, for use in diagnostics.
func Names() []string {
	out := make([]string, 0, len(All))
	for _, r := range All {
		out = append(out, string(r))
	}
	return out
}
