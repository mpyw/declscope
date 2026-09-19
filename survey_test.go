package declscope_test

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/declscope"
	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/config"
	"github.com/mpyw/declscope/internal/measure"
	"github.com/mpyw/declscope/internal/rule"
)

// TestSurveyAgreesWithTheAnalyzer is the invariant the survey rests on: per
// rule and per package variant, what it counts as reported is what the
// analyzer reports.
//
// It is deliberately not "the same number of lines as declscope ./... prints".
// The driver deduplicates the two variants of a package on the rendered
// message, and a boundary message names the first offending namespace, whose
// order differs between variants — so a line count could only be matched by
// reproducing a dedup keyed on diagnostic text, which is the coupling the
// survey exists to remove. The claim here is per variant, which is where the
// survey and the analyzer see the same package.
//
// Every rule in rule.All is compared, including the two that carry no baseline
// key and are settled in a second pass. What matters is that no rule is counted
// in one place and not in the other.
func TestSurveyAgreesWithTheAnalyzer(t *testing.T) {
	// Packages chosen for coverage of the states, not for size: a plain
	// boundary crossing, members taking their type's namespace, a package
	// whose findings are absorbed by a baseline, ignores at both levels, the
	// naming rule on and off, and the rule switched off entirely.
	for _, pkg := range []string{
		"pkglevel",
		"members",
		"baselined",
		"ignorescope",
		"qualifyrule",
		"allowboundary",
		"corens",
		// Directive problems are settled in a second pass, after every other
		// finding: a misplaced directive, a block-level ignore judged once for
		// every spec it reaches, and an ignore only the test variant needs.
		"directives",
		"blockignore",
		"braceignore",
	} {
		t.Run(pkg, func(t *testing.T) {
			for _, res := range analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, pkg) {
				if res.Pass == nil || len(res.Pass.Files) == 0 {
					continue
				}
				opts, _, err := config.Resolve(surveyTestPackageDir(res.Pass), "")
				if err != nil {
					t.Fatal(err)
				}

				reported := map[rule.Rule]int{}
				for _, d := range res.Diagnostics {
					reported[rule.Rule(d.Category)]++
				}

				surveyed := internal.Survey(res.Pass, opts)
				for _, r := range rule.All {
					count := surveyed.Findings[r]
					if count.Reported != reported[r] {
						t.Errorf("%s: %s reported %d, the analyzer reported %d",
							res.Pass.Pkg.Path(), r, count.Reported, reported[r])
					}
					if got := count.Ignored + count.Baselined + count.Reported; got != count.Found {
						t.Errorf("%s: %s found %d, but ignored+baselined+reported is %d",
							res.Pass.Pkg.Path(), r, count.Found, got)
					}
				}
			}
		})
	}
}

// TestSurveyNamesEveryCrossingItsNamespaces checks the shape of the edge set
// against the diagnostics: every namespace a crossing names must be one the
// survey also lists, since a namespace spelled in only one of the two would
// be a name the reader cannot look up.
func TestSurveyNamesEveryCrossingItsNamespaces(t *testing.T) {
	for _, res := range analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "pkglevel") {
		if res.Pass == nil || len(res.Pass.Files) == 0 {
			continue
		}
		opts, _, err := config.Resolve(surveyTestPackageDir(res.Pass), "")
		if err != nil {
			t.Fatal(err)
		}
		surveyed := internal.Survey(res.Pass, opts)
		if len(surveyed.Edges) == 0 {
			t.Fatalf("%s declares crossings but the survey found none", res.Pass.Pkg.Path())
		}

		known := map[string]bool{}
		for _, ns := range surveyed.Namespaces {
			known[ns.Name] = true
		}
		for _, e := range surveyed.Edges {
			if !known[e.From] || !known[e.To] {
				t.Errorf("edge %s -> %s names a namespace the survey does not list", e.From, e.To)
			}
			if e.Uses < 1 {
				t.Errorf("edge %s -> %s on %s carries no use", e.From, e.To, e.Declaration)
			}
			if e.State == measure.EdgeState("") {
				t.Errorf("edge %s -> %s on %s carries no state", e.From, e.To, e.Declaration)
			}
		}
	}
}

func surveyTestPackageDir(pass *analysis.Pass) string {
	return filepath.Dir(pass.Fset.Position(pass.Files[0].Pos()).Filename)
}
