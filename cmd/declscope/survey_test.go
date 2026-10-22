package main_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
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

// TestSurveyNamesAPackageOfTestFilesAloneWithoutTests checks that a package
// with nothing left to measure under -test=false is named, not dropped. The go
// command reports no error for it, so neither the rows nor the type-check count
// would show that it was matched at all.
func TestSurveyNamesAPackageOfTestFilesAloneWithoutTests(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n\nfunc Run() int { return 1 }\n")
	writeTree(t, dir, "q/q_test.go", "package q\n\nimport \"testing\"\n\nfunc TestQ(t *testing.T) {}\n")

	out, code := runIn(t, bin, dir, "survey", "-test=false", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	if !strings.Contains(out, "declscope survey: warning: example.com/declscopetest/q has only test files") {
		t.Errorf("survey dropped the package of test files without a word:\n%s", out)
	}

	// With tests, the same package is measured, and nothing is said.
	out, code = runIn(t, bin, dir, "survey", "./...")
	if code != 0 || strings.Contains(out, "warning") {
		t.Errorf("exited %d, want 0 with no warning:\n%s", code, out)
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
	// The go command's error has no position, so none is printed.
	if strings.Contains(out, "./nope: -: ") {
		t.Errorf("the refusal prints the go command's empty position:\n%s", out)
	}
}

// TestSurveyCountsADecisionPerDeclaration checks the state a package row calls
// declared. One file-level //declscope:shared can settle the scope of an
// unexported declaration and settle nothing for the exported one beside it,
// which already had shared scope whatever the config said. Counting the
// directive rather than the declarations under it made a file of exported
// names read as a file of decisions.
func TestSurveyCountsADecisionPerDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/util.go", "//declscope:shared\n\npackage p\n\nfunc utilHelper() int { return 1 }\n\nfunc UtilExported() int { return 2 }\n")
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
	if !strings.Contains(out, "- `example.com/declscopetest/p: ") {
		t.Errorf("the Markdown does not name the package that failed:\n%s", out)
	}
	if strings.Contains(out, "`example.com/declscopetest/p` |") {
		t.Errorf("a package that did not compile was given a clean row:\n%s", out)
	}
}

// TestFailedPackageEntryNamesTheTypeError pins the shape of a failed-package
// entry as the go command really produces it. The first error go/packages
// reports is the go command's own build output: a "-" position, the "# path"
// header, a newline, and a position relative to the working directory. The
// type checker's error follows it, with an absolute position on one line, and
// that is the one to name. The measure goldens are built from fixture data, so
// only a run of the binary can tell the two apart.
func TestFailedPackageEntryNamesTheTypeError(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n\nfunc broken() int { return undefinedThing }\n")

	const prefix = "example.com/declscopetest/p: "
	const suffix = "p.go:3:28: undefined: undefinedThing"
	checkEntry := func(how, entry string) {
		t.Helper()
		pos, ok := strings.CutPrefix(entry, prefix)
		if !ok || !strings.HasSuffix(entry, suffix) || strings.Contains(entry, "\n") || !filepath.IsAbs(pos) {
			t.Errorf("%s: the entry is not the type error at an absolute position:\n%q", how, entry)
		}
	}

	out, code := runIn(t, bin, dir, "survey", "-allow-errors", "-format=json", "./p")
	if code != 0 {
		t.Fatalf("-allow-errors exited %d\n%s", code, out)
	}
	var got struct {
		Checks struct {
			TypeCheck struct {
				Failed []string `json:"failed"`
			} `json:"typeCheck"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(got.Checks.TypeCheck.Failed) != 1 {
		t.Fatalf("want one failed package, got %q", got.Checks.TypeCheck.Failed)
	}
	checkEntry("survey -format=json", got.Checks.TypeCheck.Failed[0])

	out, _ = runIn(t, bin, dir, "survey", "-allow-errors", "./p")
	_, item, _ := strings.Cut(out, "\n- `")
	item, _, _ = strings.Cut(item, "`\n")
	checkEntry("survey Markdown", item)

	// The refusals indent each entry under their first line. An entry of two
	// lines would leave its second flush left, read as a line of its own.
	for _, command := range []string{"survey", "inspect"} {
		out, code := runIn(t, bin, dir, command, "./p")
		if code == 0 {
			t.Fatalf("%s measured a package that does not compile\n%s", command, out)
		}
		_, entry, _ := strings.Cut(out, "\n  ")
		entry, _, _ = strings.Cut(entry, "\n")
		checkEntry(command+" refusal", entry)
	}
}

// TestPackageWithAFailingDependencyIsCountedAsFailed checks that a package is
// not measured when a package it imports, directly or not, does not compile.
// Its own syntax type-checks against what the dependency's declarations claim,
// which is no clean result, and the analyzer itself skips such a package. So
// does baseline, and survey and inspect have to agree with both.
//
// The dependency is not a root, so the go command's build output is all there
// is to name it by: go/packages type-checks no function body outside the
// packages it was asked for.
func TestPackageWithAFailingDependencyIsCountedAsFailed(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "b/b.go", "package b\n\nfunc Broken() int { return undefinedThing }\n")
	writeTree(t, dir, "c/c.go", "package c\n\nimport \"example.com/declscopetest/b\"\n\nfunc Middle() int { return b.Broken() }\n")
	writeTree(t, dir, "a/a.go", "package a\n\nimport \"example.com/declscopetest/c\"\n\nfunc Top() int { return c.Middle() }\n")

	const prefix = "example.com/declscopetest/a: imports example.com/declscopetest/b: "
	const suffix = "b.go:3:28: undefined: undefinedThing"
	checkEntry := func(how, entry string) {
		t.Helper()
		pos, ok := strings.CutPrefix(entry, prefix)
		if !ok || !strings.HasSuffix(entry, suffix) || strings.Contains(entry, "\n") || !filepath.IsAbs(pos) {
			t.Errorf("%s: the entry does not name the failing dependency's error on one line:\n%q", how, entry)
		}
	}

	out, code := runIn(t, bin, dir, "survey", "-allow-errors", "-format=json", "./a")
	if code != 0 {
		t.Fatalf("-allow-errors exited %d\n%s", code, out)
	}
	var got struct {
		Checks struct {
			TypeCheck struct {
				Packages int      `json:"packages"`
				Failed   []string `json:"failed"`
			} `json:"typeCheck"`
		} `json:"checks"`
		Packages []struct {
			Path string `json:"path"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Checks.TypeCheck.Packages != 1 || len(got.Checks.TypeCheck.Failed) != 1 {
		t.Fatalf("want one package, failed; got %+v", got.Checks.TypeCheck)
	}
	checkEntry("survey -format=json", got.Checks.TypeCheck.Failed[0])
	if len(got.Packages) != 0 {
		t.Errorf("a package that did not type-check was given a row: %+v", got.Packages)
	}

	for _, command := range []string{"survey", "inspect"} {
		out, code := runIn(t, bin, dir, command, "./a")
		if code == 0 {
			t.Fatalf("%s measured a package whose dependency does not compile\n%s", command, out)
		}
		_, entry, _ := strings.Cut(out, "\n  ")
		entry, _, _ = strings.Cut(entry, "\n")
		checkEntry(command+" refusal", entry)
	}
	if out, code := runIn(t, bin, dir, "baseline", "./a"); code == 0 {
		t.Errorf("baseline wrote entries for a package whose dependency does not compile\n%s", out)
	}

	// Asked for together, each package is named for its own reason: c by the
	// package it imports, and b by its own type error.
	out, _ = runIn(t, bin, dir, "survey", "-allow-errors", "-format=json", "./...")
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if f := got.Checks.TypeCheck.Failed; len(f) != 3 ||
		!strings.HasPrefix(f[1], "example.com/declscopetest/b: /") ||
		!strings.HasPrefix(f[2], "example.com/declscopetest/c: imports example.com/declscopetest/b: /") {
		t.Errorf("the entries do not name each package's own reason:\n%q", f)
	}
}

// TestFailedCgoPackageEntryNamesTheCError checks that a cgo package whose C
// code does not compile is named by the C compiler's message. The type
// checker's only error there is "could not import C", which says nothing about
// the code. It needs a C toolchain, so it is skipped where cgo is off.
func TestFailedCgoPackageEntryNamesTheCError(t *testing.T) {
	if out, err := exec.Command("go", "env", "CGO_ENABLED").Output(); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Skip("cgo is not enabled")
	}
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "cg/cg.go", "package cg\n\n// int f(void) { return undefined_c; }\nimport \"C\"\n\nfunc F() int { return int(C.f()) }\n")
	writeTree(t, dir, "u/u.go", "package u\n\nimport \"example.com/declscopetest/cg\"\n\nvar _ = cg.F\n")

	out, code := runIn(t, bin, dir, "survey", "-allow-errors", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("-allow-errors exited %d\n%s", code, out)
	}
	var got struct {
		Checks struct {
			TypeCheck struct {
				Failed []string `json:"failed"`
			} `json:"typeCheck"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	prefixes := []string{"example.com/declscopetest/cg: ", "example.com/declscopetest/u: imports example.com/declscopetest/cg: "}
	if len(got.Checks.TypeCheck.Failed) != len(prefixes) {
		t.Fatalf("want cg and u failed, got %q", got.Checks.TypeCheck.Failed)
	}
	for i, entry := range got.Checks.TypeCheck.Failed {
		pos, ok := strings.CutPrefix(entry, prefixes[i])
		if !ok || !strings.Contains(entry, "cg.go:3:") || !strings.Contains(entry, "undefined_c") || strings.Contains(entry, "\n") || !filepath.IsAbs(pos) {
			t.Errorf("the entry does not name the C compiler's error on one line:\n%q", entry)
		}
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
	writeTree(t, dir, "p/a.go", "package p\n\n//declscope:shared\nfunc wide() int { return 1 }\n")
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
	for line := range strings.SplitSeq(out, "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 2 || strings.Trim(strings.TrimSpace(cells[0]), "`") != rule {
			continue
		}
		return strings.TrimSpace(cells[1]) == found
	}
	return false
}

// TestSurveyNamesTheChecksInForce covers the header the report opens with: the
// config chain that governed the packages, the rules it left on, and the
// baseline the analyzer would have read. A run that measured a package under a
// baseline and did not say so would report a clean subtree without saying what
// was suppressing it.
func TestSurveyNamesTheChecksInForce(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  surplus: off\n  unused: strict\n  naming:\n    qualify: always\n")
	writeTree(t, dir, ".declscope-baseline.yaml",
		"packages:\n  example.com/declscopetest/p:\n    boundary:\n      user: [userHelper]\n")
	writeTree(t, dir, "p/user.go", "package p\n\nfunc userHelper() int { return 1 }\n")
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderTotal() int { return userHelper() }\n")

	out, code := runIn(t, bin, dir, "survey", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	var got struct {
		Checks struct {
			Configs []struct {
				Chain    []string `json:"chain"`
				Packages int      `json:"packages"`
				Rules    struct {
					Boundary bool   `json:"boundary"`
					Qualify  string `json:"qualify"`
					Surplus  string `json:"surplus"`
					Unused   string `json:"unused"`
				} `json:"rules"`
			} `json:"configs"`
			Baselines []struct {
				Path    string `json:"path"`
				Entries int    `json:"entries"`
			} `json:"baselines"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	if len(got.Checks.Configs) != 1 {
		t.Fatalf("got %d config chains, want 1:\n%s", len(got.Checks.Configs), out)
	}
	cfg := got.Checks.Configs[0]
	if len(cfg.Chain) != 1 || !strings.HasSuffix(cfg.Chain[0], ".declscope.yaml") {
		t.Errorf("chain = %v, want the one config file", cfg.Chain)
	}
	if cfg.Rules.Qualify != "always" || cfg.Rules.Surplus != "off" || cfg.Rules.Unused != "strict" || !cfg.Rules.Boundary {
		t.Errorf("rules = %+v, want qualify always with surplus off, unused strict and boundary on", cfg.Rules)
	}

	if len(got.Checks.Baselines) != 1 {
		t.Fatalf("got %d baselines, want the one the packages resolve to:\n%s", len(got.Checks.Baselines), out)
	}
	if n := got.Checks.Baselines[0].Entries; n != 1 {
		t.Errorf("the baseline reads as %d entries, want 1", n)
	}
}

// TestSurveyRendersTheChecksAsMarkdown checks the default format on the same
// ground. A rule the config turned off reads as off rather than as absent, and
// a rule that was never asked prints dashes rather than zeros: zero findings
// and a rule nobody ran are the two readings a reader must not confuse.
func TestSurveyRendersTheChecksAsMarkdown(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  surplus: off\n")
	writeTree(t, dir, "p/user.go", "package p\n\nfunc userHelper() int { return 1 }\n")
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderTotal() int { return userHelper() }\n")

	// No pattern: the subcommand measures ./... the way the analyzer does.
	out, code := runIn(t, bin, dir, "survey")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	if !strings.Contains(out, "surplus off") {
		t.Errorf("a rule the config turned off should read as off:\n%s", out)
	}
	if !strings.Contains(out, "unused strict") {
		t.Errorf("the unused rule's mode should be named, strict by default:\n%s", out)
	}
	for _, rule := range []string{"qualify", "surplus"} {
		if !strings.Contains(out, "`"+rule+"`") {
			t.Errorf("the findings table does not list %s:\n%s", rule, out)
		}
	}
}

// TestSurveyUnusedOff checks that rules.unused: off is named in the checks
// in force, and that the unused rule then reads as never asked, with dashes,
// while the directive rule still counts what it finds.
func TestSurveyUnusedOff(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  unused: off\n")
	writeTree(t, dir, "p/user.go", "package p\n\n//declscope:ignore boundary\n//declscope:bogus\nfunc userHelper() int { return 1 }\n\nvar _ = userHelper\n")

	out, code := runIn(t, bin, dir, "survey")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	for _, want := range []string{
		"unused off",
		"| `unused`    |     - |       - |         - |        - |",
		"| `directive` |     1 |       0 |         - |        1 |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
}

// TestSurveyTakesTheConfigItWasGiven checks that -config replaces the chain
// rather than adding to it: the caller named the rules to measure under, so
// the report names that one file and no other.
func TestSurveyTakesTheConfigItWasGiven(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
	writeTree(t, dir, "chosen.yaml", "rules:\n  naming:\n    qualify: never\n")
	writeTree(t, dir, "p/user.go", "package p\n\nfunc userHelper() int { return 1 }\n")
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderTotal() int { return userHelper() }\n")

	out, code := runIn(t, bin, dir, "survey", "-config=chosen.yaml", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	var got struct {
		Checks struct {
			Configs []struct {
				Chain []string `json:"chain"`
				Rules struct {
					Qualify string `json:"qualify"`
				} `json:"rules"`
			} `json:"configs"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(got.Checks.Configs) != 1 {
		t.Fatalf("got %d config chains, want 1:\n%s", len(got.Checks.Configs), out)
	}
	if chain := got.Checks.Configs[0].Chain; len(chain) != 1 || chain[0] != "chosen.yaml" {
		t.Errorf("chain = %v, want just the file -config named", chain)
	}
	if q := got.Checks.Configs[0].Rules.Qualify; q != "never" {
		t.Errorf("qualify = %q, want the chosen config's never", q)
	}
}

// TestSurveyRefusesAnUnknownFormat checks that a format nobody renders is
// refused before any package is loaded, rather than falling back to the
// default and printing something the caller cannot parse.
func TestSurveyRefusesAnUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n\nfunc Run() int { return 1 }\n")

	out, code := runIn(t, bin, dir, "survey", "-format=toml", "./...")
	if code == 0 {
		t.Fatalf("survey accepted an unknown format:\n%s", out)
	}
	if !strings.Contains(out, "toml") {
		t.Errorf("the refusal does not name the format:\n%s", out)
	}
}

// TestSurveyRefusesABrokenConfig checks that a config one package cannot load
// stops the run with that config's error. The packages are measured in
// parallel, so this is also the check that an error from one of them is not
// lost among the results of the others.
func TestSurveyRefusesABrokenConfig(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "a/a.go", "package a\n")
	writeTree(t, dir, "b/b.go", "package b\n")
	writeTree(t, dir, "b/.declscope.yaml", "rules:\n  surplus: bogus\n")

	out, code := runIn(t, bin, dir, "survey", "./...")
	if code == 0 || !strings.Contains(out, `rules.surplus: unknown mode "bogus"`) {
		t.Errorf("want the config error and a non-zero exit, got %d:\n%s", code, out)
	}
}

// TestSurveyCountsWhatIgnoresAnswer checks that a finding an ignore silences
// is counted as ignored under its own rule: an unused report and a directive
// report, each answered at the file level by an ignore naming that rule, and
// a naming finding answered on the declaration, which the package row shows
// as exempt.
func TestSurveyCountsWhatIgnoresAnswer(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
	writeTree(t, dir, "p/misc.go", "//declscope:ignore unused\n//declscope:ignore directive\n\npackage p\n\n//declscope:private\nfunc init() {}\n\n"+
		"//declscope:bogus\n//declscope:ignore qualify\nfunc stray() int { return 1 }\n\nvar _ = stray\n")

	out, code := runIn(t, bin, dir, "survey", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	var got struct {
		Totals map[string]struct {
			Found   int `json:"found"`
			Ignored int `json:"ignored"`
		} `json:"totals"`
		Packages []struct {
			Qualify struct {
				Exempt int `json:"exempt"`
			} `json:"qualify"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	for _, rule := range []string{"unused", "directive", "qualify"} {
		if c := got.Totals[rule]; c.Found != 1 || c.Ignored != 1 {
			t.Errorf("%s: found %d, ignored %d; want one of each", rule, c.Found, c.Ignored)
		}
	}
	if len(got.Packages) != 1 || got.Packages[0].Qualify.Exempt != 1 {
		t.Errorf("packages = %+v, want one with an exempt name", got.Packages)
	}
}

// TestSurveyCountsABaselinedName checks that a naming finding the baseline
// absorbs is counted as baselined in the package row, not as reported.
func TestSurveyCountsABaselinedName(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
	writeTree(t, dir, ".declscope-baseline.yaml",
		"packages:\n  example.com/declscopetest/p:\n    qualify:\n      user: [helper]\n")
	writeTree(t, dir, "p/user.go", "package p\n\nfunc helper() int { return 1 }\n\nvar _ = helper\n")

	out, code := runIn(t, bin, dir, "survey", "-format=json", "./...")
	if code != 0 {
		t.Fatalf("survey exited %d\n%s", code, out)
	}
	var got struct {
		Packages []struct {
			Qualify struct {
				Reported  int `json:"reported"`
				Baselined int `json:"baselined"`
			} `json:"qualify"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(got.Packages) != 1 || got.Packages[0].Qualify.Baselined != 1 || got.Packages[0].Qualify.Reported != 0 {
		t.Errorf("packages = %+v, want one baselined name and none reported", got.Packages)
	}
}
