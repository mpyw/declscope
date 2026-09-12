package declscope_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/declscope"
)

func TestSuggestedFixes(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixes")
}

// TestSuggestedFixInlineStruct checks the directive fix for a field that
// shares its line with the struct type, which has to be broken onto a line of
// its own for the directive to bind to it.
func TestSuggestedFixInlineStruct(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixinline")
}

// TestSuggestedFixQualify checks the rename offered by the label rule. It
// never changes a declaration's reach, so it cannot conflict with the
// directive fix the way the old name-driven rename could.
func TestSuggestedFixQualify(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixqualify")
}

// TestSuggestedFixEmbedded checks that renaming a type also rewrites the
// idents that embed it, and the selections that reach the embedded field by
// the type's name.
func TestSuggestedFixEmbedded(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixembedded")
}

// The tests below pin the conditions under which a rename is withheld. A
// rename is offered only when it provably changes nothing but the spelling;
// in every other case the violation is still reported, without a fix. Each
// package also carries one declaration that is renamed, so the golden shows
// the guard is precise rather than merely off.

// TestSuggestedFixCapture checks that a new name bound at a reference by a
// parameter or a local, or in file scope by an import anywhere in the
// package, withholds the rename. A local declared after the reference does
// not bind it, and does not.
func TestSuggestedFixCapture(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixcapture")
}

// TestSuggestedFixPredeclared checks that unqualify never renames into a
// predeclared name, which would shadow the builtin for the whole package.
func TestSuggestedFixPredeclared(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixpredeclared")
}

// TestSuggestedFixSiblings checks that two fixes in one pass cannot rename
// two declarations to the same name: the first claims it, the second is
// reported without a fix.
func TestSuggestedFixSiblings(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixsiblings")
}

// TestSuggestedFixOutside checks that a declaration referenced from a
// generated or excluded file, which the pass does not rewrite, is not renamed.
func TestSuggestedFixOutside(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixoutside")
}

// TestSuggestedFixLinkname checks that a declaration named by a
// //go:linkname directive, which a rename cannot follow, is not renamed.
func TestSuggestedFixLinkname(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixlinkname")
}

// TestSuggestedFixTestVariant checks that the non-test variant of a package
// with in-package tests offers no rename, since it cannot see what the test
// files declare, and that the test variant, which can, still does.
func TestSuggestedFixTestVariant(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixtestvariant")
}
