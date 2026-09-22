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
		// finding: a misplaced directive, and a block-level ignore judged once
		// for every spec it reaches.
		"directives",
		"blockignore",
		"braceignore",
		// The remaining two rules, which no fixture above exercises: surplus
		// with and without a baseline absorbing it, and the one filter report.
		"surplus",
		"surplusbaselined",
		"filtercancelled/sub",
		// surplus under strict, with ignores, with a baseline, and in a
		// package whose two variants disagree about whether it may report.
		"surplusstrict",
		"surplusstrictbaselined",
		"surplusstricttests",
		// The only fixture whose two variants disagree. Everything above has
		// no _test.go file, so "per variant" goes untested without it.
		"testonlyignore",
	} {
		t.Run(pkg, func(t *testing.T) {
			for _, res := range surveyTestRun(t, pkg) {
				reported := map[rule.Rule]int{}
				for _, d := range res.Diagnostics {
					reported[rule.Rule(d.Category)]++
				}

				surveyed := surveyTestMeasure(t, res.Pass)
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

// TestSurveyEdgesAgreeWithTheCounts ties the crossings to the tally they are
// folded from: a boundary finding is per declaration, and an edge is per
// declaration and reaching namespace, so the declarations carrying edges in
// one state must be exactly as many as the rule counted in that state.
//
// Without this the state on an edge is unchecked: returning Open where the
// code returns Declared type-checks, passes every other test, and quietly
// empties both the crossing table and the most-reached table, since open
// crossings are left out of each.
func TestSurveyEdgesAgreeWithTheCounts(t *testing.T) {
	for _, pkg := range []string{"pkglevel", "baselined", "ignorescope", "allowboundary", "surplus"} {
		t.Run(pkg, func(t *testing.T) {
			for _, res := range surveyTestRun(t, pkg) {
				surveyed := surveyTestMeasure(t, res.Pass)
				count := surveyed.Findings[rule.Boundary]

				declarations := map[measure.EdgeState]map[string]bool{}
				for _, e := range surveyed.Edges {
					if declarations[e.State] == nil {
						declarations[e.State] = map[string]bool{}
					}
					declarations[e.State][e.To+"\x00"+e.Declaration] = true
				}
				for state, want := range map[measure.EdgeState]int{
					measure.EdgeReported:  count.Reported,
					measure.EdgeBaselined: count.Baselined,
					measure.EdgeIgnored:   count.Ignored,
				} {
					if got := len(declarations[state]); got != want {
						t.Errorf("%s: %d declarations carry %s edges, but the boundary rule counted %d",
							res.Pass.Pkg.Path(), got, state, want)
					}
				}
			}
		})
	}
}

// TestSurveyProducesEveryEdgeState checks that the fixtures between them reach
// every state a crossing can end in. A state nothing produces is a state
// nothing tests, and each of these is a different answer to "was this decided,
// and by whom".
func TestSurveyProducesEveryEdgeState(t *testing.T) {
	seen := map[measure.EdgeState]string{}
	for _, pkg := range []string{"pkglevel", "baselined", "ignorescope", "allowboundary", "surplus", "exportedscope"} {
		for _, res := range surveyTestRun(t, pkg) {
			for _, e := range surveyTestMeasure(t, res.Pass).Edges {
				seen[e.State] = pkg
			}
		}
	}
	for _, state := range []measure.EdgeState{
		measure.EdgeDeclared,
		measure.EdgeBaselined,
		measure.EdgeReported,
		measure.EdgeIgnored,
		measure.EdgeOpen,
		measure.EdgeUnchecked,
	} {
		if seen[state] == "" {
			t.Errorf("no fixture produced a %s crossing", state)
		}
	}
}

// TestSurveyNamesEveryCrossingItsNamespaces checks the shape of the edge set
// against the diagnostics: every namespace a crossing names must be one the
// survey also lists, since a namespace spelled in only one of the two would
// be a name the reader cannot look up.
func TestSurveyNamesEveryCrossingItsNamespaces(t *testing.T) {
	for _, res := range surveyTestRun(t, "pkglevel") {
		surveyed := surveyTestMeasure(t, res.Pass)
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

func surveyTestRun(t *testing.T, pkg string) []*analysistest.Result {
	t.Helper()
	var out []*analysistest.Result
	for _, res := range analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, pkg) {
		if res.Pass != nil && len(res.Pass.Files) > 0 {
			out = append(out, res)
		}
	}
	return out
}

// surveyTestMeasure resolves the options the analyzer would have resolved for
// this package, so the survey is measured under the same rules the diagnostics
// were produced under.
func surveyTestMeasure(t *testing.T, pass *analysis.Pass) measure.Package {
	t.Helper()
	dir := filepath.Dir(pass.Fset.Position(pass.Files[0].Pos()).Filename)
	opts, _, err := config.Resolve(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	return internal.Survey(pass, opts)
}
