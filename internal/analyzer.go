// Package internal implements the declscope analysis.
//
// The analysis answers two different questions about two kinds of declaration.
//
// Package-level identifiers compete in one flat namespace, so the problem
// there is namespace pollution. An unexported identifier is private to its
// file's namespace unless a directive widens it, and the naming rule asks it
// to carry that namespace somewhere in its name, so that the owning unit is
// legible at every use site. The namespace marks ownership. It never grants
// reach.
//
// Methods and struct fields are already namespaced by the type that owns them
// and cannot collide with anything, so the problem there is not pollution but
// encapsulation: Go makes every unexported member visible to the whole
// package, with no way to say otherwise. Members are exempt from the naming
// rule, and their bound is the namespace of the type, not of the file.
//
//declscope:core
package internal

import (
	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/baseline"
)

// Run performs the analysis for one package.
func Run(pass *analysis.Pass, opts Options) (any, error) {
	build(pass, opts).report(pass, opts)
	return nil, nil
}

// Collect returns every violation in the package, ignoring any configured
// baseline. It is the entry point used to regenerate a baseline.
func Collect(pass *analysis.Pass, opts Options) []baseline.Key {
	return build(pass, opts).keysForReport(pass, opts)
}

// build always returns a collection, empty or not. A package the filter left
// with nothing to read still has one thing to say — that the filter is why —
// and returning nil for it would drop the report along with the work.
func build(pass *analysis.Pass, opts Options) *collection {
	c := collectFiles(pass, opts)
	if len(c.files) == 0 {
		return c
	}
	c.collectTargets(pass, opts)
	c.collectRefs(pass)
	return c
}
