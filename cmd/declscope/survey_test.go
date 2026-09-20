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
// measured as one. The crossing is reached from an in-package test file, so
// the variant that sees it and the variant that does not disagree, and the
// wider one is the answer.
func TestSurveyCountsAPackageOnce(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/user.go", "package p\n\nfunc userHelper() int { return 1 }\n")
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderTotal() int { return userHelper() }\n")
	writeTree(t, dir, "p/user_test.go", "package p\n\nimport \"testing\"\n\nfunc TestUserHelper(t *testing.T) { _ = userHelper() }\n")

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
	if n := got.Totals["boundary"].Reported; n != 1 {
		t.Errorf("boundary reported %d, want 1 — a crossing counted once, not once per variant", n)
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
	if strings.Contains(out, "example.com/declscopetest/p   0") {
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
	for _, want := range []string{"unknown format", "text, json, markdown"} {
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
	if !surveyTestRow(surveyed, "surplus", "-") {
		t.Errorf("survey reports the surplus rule as asked where the analyzer stood it down:\n%s", surveyed)
	}
}

// surveyTestRow reports whether the named row of a text table starts with the
// given cell. Columns are aligned with padding that moves as other rows grow,
// so the check reads the cells rather than the spacing between them.
func surveyTestRow(out, name, first string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == name {
			return fields[1] == first
		}
	}
	return false
}
