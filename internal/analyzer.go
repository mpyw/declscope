// Package internal implements the declscope analysis.
//
// The analysis answers two different questions with two different rules.
//
// Package-level identifiers compete in one flat namespace, so the problem
// there is namespace pollution and the rule is name-driven: an unexported
// identifier is private to its file's namespace unless its name carries that
// namespace as a prefix, the way pub(crate) works in Rust.
//
// Methods and struct fields are already namespaced by the type that owns them
// and cannot collide with anything, so the problem there is not pollution but
// encapsulation: Go makes every unexported member visible to the whole
// package, with no way to say otherwise. The rule is therefore boundary-driven
// rather than name-driven, and the bound is the namespace of the type, not of
// the file.
package internal

import (
	"golang.org/x/tools/go/analysis"
)

// Run performs the analysis for one package.
func Run(pass *analysis.Pass, opts Options) (any, error) {
	c := collectFiles(pass, opts)
	if len(c.files) == 0 {
		return nil, nil
	}
	c.collectTargets(pass, opts)
	c.collectRefs(pass)
	c.report(pass, opts)
	return nil, nil
}
