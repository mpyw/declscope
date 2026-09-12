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
	// Escape: a declaration used from outside the namespace it is private to.
	Escape Rule = "escape"
	// Prefix: an unexported package-level declaration missing its namespace
	// label.
	Prefix Rule = "prefix"
	// Demote: a namespace label present where it is not required.
	Demote Rule = "demote"
	// ForeignMethod: an unexported method grown on a type belonging to another
	// namespace.
	ForeignMethod Rule = "foreign-method"
)

// All lists every rule, in the order they are reported.
var All = []Rule{Escape, Prefix, Demote, ForeignMethod}

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
