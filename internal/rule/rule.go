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
	// Qualify: a package-level declaration whose name does not carry its
	// namespace, which namespace.Qualify fixes by prefixing.
	Qualify Rule = "qualify"
	// Surplus: a //declscope:package directive with no use from another
	// namespace visible to declscope. It reports from an absence, so it has no
	// fix — every remaining false positive would be an automatic edit deleting
	// a load-bearing directive — and it is off by default.
	Surplus Rule = "surplus"
	// Directive: a directive that binds nothing, or that is malformed or
	// misplaced. It has no fix and no configuration; it carries a name so that
	// an ignore can silence one and a baseline can record one, which a report
	// with no rule could never allow.
	Directive Rule = "directive"
	// Filter: a filter.only that cannot take effect, because an only above it
	// in the chain of config files removes everything it matches. It is about
	// the configuration rather than a declaration, so it carries no fix and no
	// baseline entry, and a file-level ignore is what silences it.
	Filter Rule = "filter"
)

// All lists every rule, in the order they are reported.
var All = []Rule{Boundary, Qualify, Surplus, Directive, Filter}

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
