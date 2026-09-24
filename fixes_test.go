package declscope_test

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/declscope"
	"github.com/mpyw/declscope/internal/rule"
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

// TestSuggestedFixQualify checks the rename offered by the naming rule. It
// never changes a declaration's reach, so it cannot conflict with the
// directive fix.
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
// not bind it, and withholds nothing.
func TestSuggestedFixCapture(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixcapture")
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

// TestSuggestedFixExcluded checks that a declaration named from a file the
// build configuration excludes is not renamed, in either direction: the
// excluded file names load, and already declares the name spare would take.
// Neither is rewritten by the fix, and no variant of this package ever sees
// the file. A declaration it does not mention is still renamed.
func TestSuggestedFixExcluded(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixexcluded")
}

// TestSuggestedFixMembers checks that two directive insertions in one pass
// cannot converge. A directive inserted on a type reaches the type's members,
// so a member whose type is widened in the same run is reported without a fix
// of its own; a member whose type is not gets one.
func TestSuggestedFixMembers(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixmembers")
}

// TestSuggestedFixSurplusStrict checks strict's fix: //declscope:private above
// the declaration, after a bare comment line under a doc comment, once for an
// entry holding two names, and on a line of its own for a field that shared
// one. A directive that narrowing would leave binding nothing withholds the
// fix, so that file takes no edit. And a boundary fix on a type narrows, in
// the same edit, each member the widened type would otherwise leave wide.
func TestSuggestedFixSurplusStrict(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixsurplusstrict")
}

// TestSuggestedFixDirectiveStrict checks the fix rules.directive: strict
// offers: it deletes the redundant directive, whether it stands on its own
// line or trails a field, with the bare // that separated it from a doc
// comment, and with the blank line after a file-level one. A directive on a
// declaration another namespace uses is reported without a fix, so that file
// takes no edit.
func TestSuggestedFixDirectiveStrict(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixdirectivestrict")
}

// TestSuggestedFixDirectiveStrictWithheld checks each reason the directive
// rule's strict fix is withheld, with the report kept: a field whose type the
// boundary fix widens (b.go), a spec whose block keeps a report the deletion
// would change (d.go, and f.go where loose alone reports the block), and a
// declaration a file the build excludes names (g.go). Only a.go and the
// boundary fix in b.go take edits; an outer directive an ignore answers does
// not withhold.
func TestSuggestedFixDirectiveStrictWithheld(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixdirectivestrictwithheld")
}

// TestSuggestedFixDirectiveStrictSurplus checks the reasons the directive
// rule's strict fix is withheld while surplus reads a //declscope:package: an
// ignore answering surplus (b.go), and a directive restating an enclosing one
// (c.go). Only a.go, where the deletion settles surplus's report too, takes
// an edit.
func TestSuggestedFixDirectiveStrictSurplus(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixdirectivestrictsurplus")
}

// fixesWantless collects what analysistest reports instead of failing, for a
// fixture no single set of want comments describes: its variants disagree by
// design.
type fixesWantless struct{ errors []string }

func (w *fixesWantless) Errorf(format string, args ...any) {
	w.errors = append(w.errors, fmt.Sprintf(format, args...))
}

// TestSuggestedFixDirectiveStrictUnseenTests checks that the ordinary variant
// of a package with in-package tests offers no deletion. Only the test
// variant sees order_test.go use userShared from another namespace, and it
// withholds the fix, since the boundary report names the directive. The
// driver applies the fixes of both variants, so an offer from the ordinary
// one would be applied anyway.
//
// The boundary report exists in the test variant alone, and a want in
// user.go must hold in both, so the diagnostics are read here directly.
func TestSuggestedFixDirectiveStrictUnseenTests(t *testing.T) {
	var w fixesWantless
	results := analysistest.Run(&w, analysistest.TestData(), declscope.Analyzer, "directivestricttests")
	const want = "unused //declscope:private on userShared: it already has private scope"
	variants := 0
	for _, r := range results {
		for _, d := range r.Diagnostics {
			if d.Message != want {
				continue
			}
			variants++
			if len(d.SuggestedFixes) > 0 {
				t.Errorf("%s offers a fix in %s, which cannot see every use", want, r.Pass.Pkg.Path())
			}
		}
	}
	if variants != 2 {
		t.Errorf("reported in %d variants, want both; analysistest said:\n%s", variants, strings.Join(w.errors, "\n"))
	}
}

// TestSuggestedFixDirectiveStrictUnreadable checks that a pass with no
// ReadFile, as the subcommands build one, still makes the strict reports and
// offers no deletion, since it cannot see the lines it would delete.
func TestSuggestedFixDirectiveStrictUnreadable(t *testing.T) {
	blind := &analysis.Analyzer{
		Name: declscope.Analyzer.Name,
		Doc:  declscope.Analyzer.Doc,
		Run: func(pass *analysis.Pass) (any, error) {
			unreadable := *pass
			unreadable.Analyzer, unreadable.ReadFile = declscope.Analyzer, nil
			return declscope.Analyzer.Run(&unreadable)
		},
	}
	for _, r := range analysistest.Run(t, analysistest.TestData(), blind, "fixdirectivestrict") {
		for _, d := range r.Diagnostics {
			if d.Category == string(rule.Directive) && len(d.SuggestedFixes) > 0 {
				t.Errorf("%q offers a fix it could not have read", d.Message)
			}
		}
	}
}

// TestSuggestedFixUnparsableExcludedFile checks that a file the build excludes
// and the pass cannot parse withholds every rename, whether its package
// clause or its body is what fails: the pass cannot tell what it names.
func TestSuggestedFixUnparsableExcludedFile(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixunseenclause", "fixunseenbody")
}

// TestSuggestedFixSurplusNarrowsWithTypeFix checks that under surplus strict a
// boundary fix on a type narrows, in the same edit, the members no other
// namespace reads.
func TestSuggestedFixSurplusNarrowsWithTypeFix(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixsurplusnarrow")
}
