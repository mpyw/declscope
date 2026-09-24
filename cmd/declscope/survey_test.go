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

// TestSurveyNamesTheChecksInForce covers the header the report opens with: the
// config chain that governed the packages, the rules it left on, and the
// baseline the analyzer would have read. A run that measured a package under a
// baseline and did not say so would report a clean subtree without saying what
// was suppressing it.
func TestSurveyNamesTheChecksInForce(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  surplus: off\n  directive: strict\n  naming:\n    qualify: always\n")
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
					Boundary  bool   `json:"boundary"`
					Qualify   string `json:"qualify"`
					Surplus   string `json:"surplus"`
					Directive string `json:"directive"`
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
	if cfg.Rules.Qualify != "always" || cfg.Rules.Surplus != "off" || cfg.Rules.Directive != "strict" || !cfg.Rules.Boundary {
		t.Errorf("rules = %+v, want qualify always with surplus off, directive strict and boundary on", cfg.Rules)
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
	if !strings.Contains(out, "directive loose") {
		t.Errorf("the directive rule's mode should be named, loose by default:\n%s", out)
	}
	for _, rule := range []string{"qualify", "surplus"} {
		if !strings.Contains(out, "`"+rule+"`") {
			t.Errorf("the findings table does not list %s:\n%s", rule, out)
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
// is counted as ignored, for a directive report answered at the file level
// and for a naming finding answered on the declaration, which the package row
// shows as exempt.
func TestSurveyCountsWhatIgnoresAnswer(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
	writeTree(t, dir, "p/misc.go", "//declscope:ignore directive\n\npackage p\n\n//declscope:private\nfunc init() {}\n\n"+
		"//declscope:ignore qualify\nfunc stray() int { return 1 }\n\nvar _ = stray\n")

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
	for _, rule := range []string{"directive", "qualify"} {
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
