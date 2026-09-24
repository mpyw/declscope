// Package rule names declscope's checks, and the modes their settings are
// spelled in.
//
// The same names are used everywhere a check has to be referred to: the
// category of a diagnostic, the key of a baseline entry, and the target of an
// ignore directive. One vocabulary means that whatever a diagnostic calls
// itself is exactly what can be written to silence or record it.
//
// This file is the package's core, and mode.go is its own namespace: the
// rule names are what every caller spells, and a named namespace would spell
// itself into each of them (rule.RuleBoundary). Everything here is exported
// and already package-scoped, so the core states no scope.
//
//declscope:core

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
	// Surplus: package scope wider than any use visible to declscope. It
	// reports from an absence, in two shapes that share the name. Under
	// rules.surplus: loose, a //declscope:package whose every dependent is
	// unreached, with no fix — every remaining false positive would be an
	// automatic edit deleting a load-bearing directive. Under strict, also
	// each unreached dependent of a directive that is otherwise in use, fixed
	// by narrowing that one declaration.
	Surplus Rule = "surplus"
	// Unused: a directive that decides nothing. A scope directive that binds
	// no declaration, or under rules.unused: strict one that names the scope
	// everything it reaches would have without it, and an ignore that
	// silenced nothing. rules.unused sets how far it goes, off included.
	Unused Rule = "unused"
	// Directive: a directive that is malformed, unknown, conflicting or
	// misplaced. It has no fix and no configuration, so it is always on. It
	// carries a name so that an ignore can silence one, which a report with no
	// rule could never allow.
	Directive Rule = "directive"
	// Filter: a filter.only that cannot take effect, because an only above it
	// in the chain of config files removes everything it matches. It is about
	// the configuration rather than a declaration, so it carries no fix and no
	// baseline entry, and a file-level ignore is what silences it.
	Filter Rule = "filter"
)

// All lists every rule, in the order they are reported.
var All = []Rule{Boundary, Qualify, Surplus, Unused, Directive, Filter}

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
