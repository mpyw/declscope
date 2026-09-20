package main_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// These drive the built binary against a temporary module, for the reason
// baseline_test.go does: the properties under test are about what the
// subcommand loads and what it then reports, and a unit test of either half
// proves nothing about the handshake between them.
//
// The one they exist for is the package count. Loading with tests yields two
// packages for one import path, and measuring both counts every finding
// twice — a change nothing else here would notice, because every other test
// works from one analysis.Pass that the harness hands it.

// TestSurveyCountsAPackageOnce checks that the two variants of a package are
// measured as one, and that the one measured is the wider.
//
// The test file is its own namespace and reaches a declaration of another, so
// the two variants genuinely disagree: without it the package holds one
// crossing, with it two. Measuring both variants would report three.
func TestSurveyCountsAPackageOnce(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/user.go", "package p\n\nfunc userHelper() int { return 1 }\n\nfunc userName() string { return \"u\" }\n")
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderTotal() int { return userHelper() }\n")
	writeTree(t, dir, "p/cart_test.go", "package p\n\nimport \"testing\"\n\nfunc TestCart(t *testing.T) { _ = userName() }\n")

	out, code := runIn(t, bin, dir, "survey", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}

	var got struct {
		Checks struct {
			TypeCheck struct {
				Packages int      `json:"packages"`
				Failed   []string `json:"failed"`
			} `json:"typeCheck"`
		} `json:"checks"`
		Totals map[string]struct {
			Found    int `json:"found"`
			Reported int `json:"reported"`
		} `json:"totals"`
		Packages []struct {
			Package  string `json:"package"`
			Boundary struct {
				Reported int `json:"reported"`
			} `json:"boundary"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	if n := got.Checks.TypeCheck.Packages; n != 1 {
		t.Errorf("type check counted %d packages, want 1 — the two variants of one package are one row", n)
	}
	if n := len(got.Packages); n != 1 {
		t.Errorf("%d package rows, want 1: %+v", n, got.Packages)
	}
	if n := got.Totals["boundary"].Reported; n != 2 {
		t.Errorf("boundary reported %d, want 2: the two crossings of the wider variant, each counted once", n)
	}
	if n := len(got.Checks.TypeCheck.Failed); n != 0 {
		t.Errorf("type check reported %d failures on a module that compiles", n)
	}
}

// TestSurveyKeepsARealMainPackageEndingInDotTest checks that the synthetic test
// executable is identified from go/packages metadata, not from an import-path
// suffix that a real package may also have.
func TestSurveyKeepsARealMainPackageEndingInDotTest(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "cmd.test/main.go", "package main\n\nfunc main() {}\n")

	out, code := runIn(t, bin, dir, "survey", "-test=false", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	var got struct {
		Packages []struct {
			Package string `json:"package"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(got.Packages) != 1 || got.Packages[0].Package != "example.com/declscopetest/cmd.test" {
		t.Errorf("the real .test package was discarded as a synthetic test main: %+v", got.Packages)
	}
}

// TestSurveyRefusesAPatternItCannotLoad checks that a pattern naming nothing
// is refused rather than answered with zeros. A package that could not be
// loaded has no syntax at all, so measuring only what is analyzable would drop
// it from the report along with the reason.
func TestSurveyRefusesAPatternItCannotLoad(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)

	out, code := runIn(t, bin, dir, "survey", "./nope")
	if code == 0 {
		t.Fatalf("survey answered for a pattern it could not load\n%s", out)
	}
	if !strings.Contains(out, "./nope") {
		t.Errorf("the refusal does not name the pattern:\n%s", out)
	}
}

// TestSurveyCountsADecisionPerDeclaration checks the state a package row calls
// declared. One file-level //declscope:package can settle the scope of an
// unexported declaration and settle nothing for the exported one beside it,
// which already had package scope whatever the config said. Counting the
// directive rather than the declarations under it made a file of exported
// names read as a file of decisions.
func TestSurveyCountsADecisionPerDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/util.go", "//declscope:package\n\npackage p\n\nfunc utilHelper() int { return 1 }\n\nfunc UtilExported() int { return 2 }\n")
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderTotal() int { return utilHelper() + UtilExported() }\n")

	out, code := runIn(t, bin, dir, "inspect", "-format=json", "./p")
	if code != 0 {
		t.Fatalf("inspect exited %d\n%s", code, out)
	}
	var got struct {
		Edges []struct {
			Declaration string `json:"declaration"`
			State       string `json:"state"`
		} `json:"edges"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	want := map[string]string{"utilHelper": "declared", "UtilExported": "open"}
	for _, e := range got.Edges {
		if w, ok := want[e.Declaration]; ok && e.State != w {
			t.Errorf("%s crosses as %q, want %q", e.Declaration, e.State, w)
		}
		delete(want, e.Declaration)
	}
	for decl := range want {
		t.Errorf("no crossing reported for %s", decl)
	}
}

// TestSurveyRefusesAPackageThatDoesNotCompile pins the refusal. A package that
// does not type-check yields no findings, which is indistinguishable from one
// with nothing wrong, so a count must not be printed for it at all.
func TestSurveyRefusesAPackageThatDoesNotCompile(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n\nfunc broken() int { return undefinedThing }\n")

	out, code := runIn(t, bin, dir, "survey", "./...")
	if code == 0 {
		t.Fatalf("survey measured a package that does not compile\n%s", out)
	}
	if !strings.Contains(out, "do not type-check") {
		t.Errorf("the refusal does not say why:\n%s", out)
	}

	// With -allow-errors the run continues, and the broken package is named
	// rather than given a clean row.
	out, code = runIn(t, bin, dir, "survey", "-allow-errors", "./...")
	if code != 0 {
		t.Fatalf("-allow-errors exited %d\n%s", code, out)
	}
	if !strings.Contains(out, "1 failed") {
		t.Errorf("the type check line does not report the failure:\n%s", out)
	}
	if strings.Contains(out, "`example.com/declscopetest/p` |") {
		t.Errorf("a package that did not compile was given a clean row:\n%s", out)
	}
}

// TestFormatNamesWhatItAccepts checks the flag answers a bad value with the
// values it takes, the way a rejected rules.naming.qualify does.
func TestFormatNamesWhatItAccepts(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n\nfunc Run() int { return 1 }\n")

	out, code := runIn(t, bin, dir, "survey", "-format=yaml", "./...")
	if code == 0 {
		t.Fatalf("an unknown format was accepted\n%s", out)
	}
	for _, want := range []string{"unknown format", "markdown, json"} {
		if !strings.Contains(out, want) {
			t.Errorf("the error does not say %q:\n%s", want, out)
		}
	}
}

// TestSurveyLeavesTheSurplusRuleAloneWhereTheAnalyzerDoes checks the one claim
// the whole command rests on: it reports what the analyzer found, not what a
// differently built pass would have found.
//
// The surplus rule stands itself down for a package holding a file it cannot
// read as a reference site. A pass built without OtherFiles cannot see that,
// and reported a directive as surplus that declscope itself refuses to report.
func TestSurveyLeavesTheSurplusRuleAloneWhereTheAnalyzerDoes(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/a.go", "package p\n\n//declscope:package\nfunc wide() int { return 1 }\n")
	writeTree(t, dir, "p/b.go", "package p\n\nfunc local() int { return wide() }\n")
	writeTree(t, dir, "p/p.s", "")

	analyzed, _ := runIn(t, bin, dir, "./p")
	surveyed, code := runIn(t, bin, dir, "survey", "./p")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, surveyed)
	}
	if strings.Contains(analyzed, "surplus") {
		t.Fatalf("the analyzer itself reported surplus here; the fixture no longer tests what it means to:\n%s", analyzed)
	}
	if !surveyTestRuleRow(surveyed, "surplus", "-") {
		t.Errorf("survey reports the surplus rule as asked where the analyzer stood it down:\n%s", surveyed)
	}
}

// surveyTestRuleRow reports whether the Findings row for a rule opens with the
// given cell. The cells are padded to their column, so the check reads them
// rather than the spacing.
func surveyTestRuleRow(out, rule, found string) bool {
	for _, line := range strings.Split(out, "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 2 || strings.Trim(strings.TrimSpace(cells[0]), "`") != rule {
			continue
		}
		return strings.TrimSpace(cells[1]) == found
	}
	return false
}
