package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestInspectNamesThePackageItWasAskedFor checks that an external test package
// is not reported in place of its subject. It declares into its own scope with
// its own namespaces, so answering with it would hand back a different package
// under the asked-for name.
func TestInspectNamesThePackageItWasAskedFor(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "q/q.go", "package q\n\nfunc Run() int { return 1 }\n")
	// Two external test files, so the external package holds more files than
	// its subject and would win any widest-variant contest.
	writeTree(t, dir, "q/a_test.go", "package q_test\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n")
	writeTree(t, dir, "q/b_test.go", "package q_test\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n")

	out, code := runIn(t, bin, dir, "inspect", "./q")
	if code != 0 {
		t.Fatalf("inspect exited %d\n%s", code, out)
	}
	if !strings.Contains(out, "# example.com/declscopetest/q\n") {
		t.Errorf("inspect reported another package:\n%s", out)
	}
}

// TestInspectRefusesMoreThanOnePackage pins the refusal a namespace's meaning
// depends on: two packages can spell one the same way and mean nothing in
// common, so their counts cannot be shown together or averaged.
func TestInspectRefusesMoreThanOnePackage(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "a/a.go", "package a\n\nfunc Run() int { return 1 }\n")
	writeTree(t, dir, "b/b.go", "package b\n\nfunc Run() int { return 1 }\n")

	out, code := runIn(t, bin, dir, "inspect", "./...")
	if code == 0 {
		t.Fatalf("inspect measured two packages at once\n%s", out)
	}
	for _, want := range []string{"matches 2 packages", "example.com/declscopetest/a", "example.com/declscopetest/b"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, out)
		}
	}
}

// TestInspectRefusesMoreThanOnePackageBeforeTypeChecking checks that the
// refusal is about the pattern even when one of the packages it names does not
// type-check. The names are counted before the full load, so the broken
// package is never type-checked.
func TestInspectRefusesMoreThanOnePackageBeforeTypeChecking(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "a/a.go", "package a\n\nfunc Run() int { return 1 }\n")
	writeTree(t, dir, "broken/broken.go", "package broken\n\nfunc Run() int { return nope }\n")

	for _, args := range [][]string{{"./..."}, {"all"}, {"./a", "./broken"}} {
		out, code := runIn(t, bin, dir, append([]string{"inspect"}, args...)...)
		if code != 1 {
			t.Fatalf("inspect %v exited %d, want 1\n%s", args, code, out)
		}
		if !strings.Contains(out, "matches 2 packages") || strings.Contains(out, "type-check") {
			t.Errorf("inspect %v refused for the wrong reason:\n%s", args, out)
		}
	}

	// A mistyped directory is not counted: its own error says what is wrong.
	out, code := runIn(t, bin, dir, "inspect", "./a", "./nope")
	if code != 1 || !strings.Contains(out, "directory not found") || strings.Contains(out, "matches") {
		t.Errorf("exited %d, want 1 with the missing directory's error:\n%s", code, out)
	}
}

// TestInspectRefusesMoreThanOnePackageFromADriverPattern checks the refusal
// after the full load. A driver's pattern can name several packages without
// a wildcard, so it skips the count before the load. The driver hands back
// two packages for one such pattern.
func TestInspectRefusesMoreThanOnePackageFromADriverPattern(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a/a.go", "package a\n")
	writeTree(t, dir, "b/b.go", "package b\n")
	response := fmt.Sprintf(`{"Compiler":"gc","Arch":"amd64","Roots":["example.com/a","example.com/b"],"Packages":[`+
		`{"ID":"example.com/a","Name":"a","PkgPath":"example.com/a","GoFiles":[%[1]q],"CompiledGoFiles":[%[1]q]},`+
		`{"ID":"example.com/b","Name":"b","PkgPath":"example.com/b","GoFiles":[%[2]q],"CompiledGoFiles":[%[2]q]}]}`,
		filepath.Join(dir, "a", "a.go"), filepath.Join(dir, "b", "b.go"))
	writeTree(t, dir, "response.json", response)
	driver := filepath.Join(dir, "driver")
	if err := os.WriteFile(driver, []byte("#!/bin/sh\ncat "+filepath.Join(dir, "response.json")+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPACKAGESDRIVER", driver)

	out, code := runIn(t, bin, dir, "inspect", "//:everything")
	if code != 1 {
		t.Fatalf("inspect exited %d, want 1\n%s", code, out)
	}
	for _, want := range []string{"matches 2 packages", "example.com/a", "example.com/b"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, out)
		}
	}
}

// TestInspectDoesNotMistakeARealTestSuffixForAnExternalTest checks that an
// ordinary package named q_test is not discarded merely because q is present.
func TestInspectDoesNotMistakeARealTestSuffixForAnExternalTest(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "q/q.go", "package q\n\nfunc Run() int { return 1 }\n")
	writeTree(t, dir, "q_test/q.go", "package q_test\n\nfunc Run() int { return 1 }\n")

	out, code := runIn(t, bin, dir, "inspect", "./...")
	if code == 0 {
		t.Fatalf("inspect discarded the real q_test package\n%s", out)
	}
	for _, want := range []string{"matches 2 packages", "example.com/declscopetest/q", "example.com/declscopetest/q_test"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, out)
		}
	}
}

// TestInspectIgnoresErrorsInTheSkippedExternalTestPackage checks that inspect
// can still report q when q_test, which it deliberately does not report, fails
// to type-check.
func TestInspectIgnoresErrorsInTheSkippedExternalTestPackage(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "q/q.go", "package q\n\nfunc Run() int { return 1 }\n")
	writeTree(t, dir, "q/q_test.go", "package q_test\n\nfunc broken() { undefinedThing() }\n")

	out, code := runIn(t, bin, dir, "inspect", "./q")
	if code != 0 {
		t.Fatalf("inspect was blocked by the external package it skips: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "# example.com/declscopetest/q\n") {
		t.Errorf("inspect did not report the requested package:\n%s", out)
	}
}

// TestInspectWithoutAPackagePrintsUsage checks the one argument inspect
// insists on. survey and baseline default to ./..., and inspect cannot: it
// answers for one package, and defaulting would pick one silently.
func TestInspectWithoutAPackagePrintsUsage(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)

	out, code := runIn(t, bin, dir, "inspect")
	if code != 2 {
		t.Fatalf("inspect with no package exited %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "inspect") || !strings.Contains(out, "-format") {
		t.Errorf("the usage does not describe the subcommand:\n%s", out)
	}
}

// TestInspectSaysNothingWasChecked covers the report for a package every file
// of which was excluded — here because all of it is generated. Its tables are
// empty, and empty tables read as a clean package, so the report opens by
// saying that nothing was read rather than letting the zeros speak.
func TestInspectSaysNothingWasChecked(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/zz_gen.go",
		"// Code generated by a tool. DO NOT EDIT.\n\npackage p\n\nfunc genHelper() int { return 1 }\n\nvar _ = genHelper\n")

	out, code := runIn(t, bin, dir, "inspect", "./p")
	if code != 0 {
		t.Fatalf("inspect exited %d\n%s", code, out)
	}
	if !strings.Contains(out, "No file of this package was read") {
		t.Errorf("a package with nothing read should say so:\n%s", out)
	}
}

// TestInspectRefusesWhatItCannotMeasure covers the three answers that are not
// a report. Each one is a refusal rather than an empty table, because a
// package that did not load, did not compile, or was never named yields the
// same zeros as a package with nothing wrong.
func TestInspectRefusesWhatItCannotMeasure(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n\nfunc Run() int { return 1 }\n")
	writeTree(t, dir, "broken/broken.go", "package broken\n\nfunc Run() int { return nope }\n")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"an unknown format", []string{"-format=toml", "./p"}, "toml"},
		{"a pattern nothing matches", []string{"./nope"}, "./nope"},
		{"a package that does not compile", []string{"./broken"}, "do not type-check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := runIn(t, bin, dir, append([]string{"inspect"}, tt.args...)...)
			if code == 0 {
				t.Fatalf("inspect answered anyway:\n%s", out)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("the refusal does not name %q:\n%s", tt.want, out)
			}
		})
	}
}

// TestInspectTakesTheConfigItWasGiven checks that -config replaces the chain
// the report names, the way it does for survey: the caller named the rules, so
// the file beside the package is not what ran.
func TestInspectTakesTheConfigItWasGiven(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "chosen.yaml", "rules:\n  naming:\n    qualify: always\n")
	writeTree(t, dir, "p/.declscope.yaml", "rules:\n  naming:\n    qualify: never\n")
	writeTree(t, dir, "p/user.go", "package p\n\nfunc helper() int { return 1 }\n\nvar _ = helper\n")

	out, code := runIn(t, bin, dir, "inspect", "-config=chosen.yaml", "./p")
	if code != 0 {
		t.Fatalf("inspect exited %d\n%s", code, out)
	}
	if !strings.Contains(out, "chosen.yaml") {
		t.Errorf("the report does not name the config it was given:\n%s", out)
	}
	if strings.Contains(out, ".declscope.yaml") {
		t.Errorf("the report names a config that did not run:\n%s", out)
	}
}

// TestInspectRefusesAPatternMatchingNothing checks a pattern that names no
// package: there is nothing to measure, and the run says so.
func TestInspectRefusesAPatternMatchingNothing(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	out, code := runIn(t, bin, dir, "inspect", "./...")
	if code != 1 || !strings.Contains(out, "declscope inspect: ./... matched no packages") {
		t.Errorf("exited %d, want 1 with the refusal:\n%s", code, out)
	}
}

// TestInspectRefusesAMissingConfig checks that a -config that is not there
// stops the run, rather than measuring under the defaults.
func TestInspectRefusesAMissingConfig(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/p.go", "package p\n")
	out, code := runIn(t, bin, dir, "inspect", "-config", filepath.Join(dir, "absent.yaml"), "./p")
	if code != 1 || !strings.Contains(out, "absent.yaml") {
		t.Errorf("exited %d, want 1 naming the file:\n%s", code, out)
	}
}

// TestInspectRefusesAPackageOfTestFilesAloneWithoutTests checks the one
// pattern that leaves nothing to measure without an error: a directory of
// test files under -test=false. The go command lists it with no files and no
// error, and the refusal must say why, not count zero packages as many.
func TestInspectRefusesAPackageOfTestFilesAloneWithoutTests(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "q/q_test.go", "package q\n\nimport \"testing\"\n\nfunc TestQ(t *testing.T) {}\n")

	out, code := runIn(t, bin, dir, "inspect", "-test=false", "./q")
	if code != 1 || !strings.Contains(out, "declscope inspect: example.com/declscopetest/q has only test files") {
		t.Errorf("exited %d, want 1 naming the package of test files:\n%s", code, out)
	}
	if strings.Contains(out, "packages") {
		t.Errorf("the refusal speaks of many packages:\n%s", out)
	}
}

// TestInspectSymbolsNamesTheReachingDeclaration checks which declaration a
// reference is charged to: the package-level one it sits in, never a type or a
// var a function body declares, with a member's references charged to its
// type and each name of a var spec to its own value.
func TestInspectSymbolsNamesTheReachingDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/user.go", "package p\n\nfunc userHelper() int { return 1 }\n\ntype userPoint struct{ x int }\n\ntype userKind int\n")
	writeTree(t, dir, "p/order.go", `package p

type orderBox struct {
	pt userPoint
}

func (b orderBox) orderSum() int { return userHelper() + userHelper() + b.pt.x }

var orderA, orderB = 2, userHelper()

var orderC, orderD userKind

func orderRun() int {
	type local struct{ k userKind }
	return int(local{}.k)
}

func init() { _ = userHelper() }
`)

	out, code := runIn(t, bin, dir, "inspect", "-format=json", "-symbols", "./p")
	if code != 0 {
		t.Fatalf("inspect exited %d\n%s", code, out)
	}
	var got struct {
		Edges []struct {
			Declaration string `json:"declaration"`
			Uses        int    `json:"uses"`
		} `json:"edges"`
		SymbolEdges []struct {
			From            string `json:"from"`
			To              string `json:"to"`
			FromDeclaration string `json:"fromDeclaration"`
			FromKind        string `json:"fromKind"`
			Declaration     string `json:"declaration"`
			Uses            int    `json:"uses"`
		} `json:"symbolEdges"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var rows []string
	for _, e := range got.SymbolEdges {
		rows = append(rows, fmt.Sprintf("%s→%s %s %s→%s %d", e.From, e.To, e.FromKind, e.FromDeclaration, e.Declaration, e.Uses))
	}
	want := []string{
		"order→user func init→userHelper 1",
		"order→user var orderB→userHelper 1",
		"order→user type orderBox→userPoint 1",
		"order→user method orderBox.orderSum→userHelper 2",
		"order→user var orderC→userKind 1",
		"order→user func orderRun→userKind 1",
	}
	// userPoint.x is reached too, from orderSum. Its row proves a member is
	// resolved on the reached side, so it is checked apart from the order.
	member := "order→user method orderBox.orderSum→userPoint.x 1"
	if !slices.Contains(rows, member) {
		t.Errorf("missing %q in\n%s", member, strings.Join(rows, "\n"))
	}
	rows = slices.DeleteFunc(rows, func(r string) bool { return r == member })
	if !slices.Equal(rows, want) {
		t.Errorf("symbol edges:\n%s\nwant:\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}

	// Split by the reaching side, the uses still add up to each edge's.
	sums := map[string]int{}
	for _, e := range got.SymbolEdges {
		sums[e.Declaration] += e.Uses
	}
	for _, e := range got.Edges {
		if sums[e.Declaration] != e.Uses {
			t.Errorf("%s: %d uses over its symbol edges, %d on its edge", e.Declaration, sums[e.Declaration], e.Uses)
		}
	}

	// Without the flag the key is absent, not empty: nothing was resolved.
	out, _ = runIn(t, bin, dir, "inspect", "-format=json", "./p")
	if strings.Contains(out, "symbolEdges") {
		t.Errorf("symbolEdges is reported without -symbols:\n%s", out)
	}
}

// TestInspectSymbolNamespacesNarrowsTheSymbols checks that -symbol-namespaces
// keeps the crossings into or out of any namespace it names, and refuses where
// it would narrow nothing: without -symbols, or naming a namespace the package
// does not have.
func TestInspectSymbolNamespacesNarrowsTheSymbols(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/a.go", "package p\n\nfunc aRun() int { return bHelper() }\n")
	writeTree(t, dir, "p/b.go", "package p\n\nfunc bHelper() int { return cHelper() }\n")
	writeTree(t, dir, "p/c.go", "package p\n\nfunc cHelper() int { return 1 }\n\nfunc cRun() int { return aRun() + dRun() }\n")
	writeTree(t, dir, "p/d.go", "package p\n\nfunc dRun() int { return bHelper() }\n")

	for _, tc := range []struct {
		flag string
		want []string
	}{
		// b → c and c → d and d → b touch neither side of a.
		{"a", []string{"aRun→bHelper", "cRun→aRun"}},
		// Any of the names, not only the crossings between them: c → d
		// touches d alone and stays.
		{"a, d,", []string{"aRun→bHelper", "cRun→aRun", "cRun→dRun", "dRun→bHelper"}},
		// A "!" leaves a namespace out, from everything when nothing is kept.
		{"!a", []string{"bHelper→cHelper", "cRun→dRun", "dRun→bHelper"}},
		// And from what is kept, whatever the kept name says: c → a touches
		// a, but it touches c too.
		{"a, !c", []string{"aRun→bHelper"}},
	} {
		out, code := runIn(t, bin, dir, "inspect", "-format=json", "-symbols", "-symbol-namespaces="+tc.flag, "./p")
		if code != 0 {
			t.Fatalf("inspect exited %d\n%s", code, out)
		}
		var got struct {
			SymbolIncludedNamespaces []string `json:"symbolIncludedNamespaces"`
			SymbolExcludedNamespaces []string `json:"symbolExcludedNamespaces"`
			SymbolEdges              []struct {
				FromDeclaration string `json:"fromDeclaration"`
				Declaration     string `json:"declaration"`
			} `json:"symbolEdges"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		var rows []string
		for _, e := range got.SymbolEdges {
			rows = append(rows, e.FromDeclaration+"→"+e.Declaration)
		}
		if !slices.Equal(rows, tc.want) {
			t.Errorf("-symbol-namespaces=%q: got %v, want %v", tc.flag, rows, tc.want)
		}
		if len(got.SymbolIncludedNamespaces)+len(got.SymbolExcludedNamespaces) == 0 {
			t.Errorf("-symbol-namespaces=%q: the JSON does not say what narrowed it", tc.flag)
		}
	}

	out, code := runIn(t, bin, dir, "inspect", "-symbol-namespaces=a", "./p")
	if code != 1 || !strings.Contains(out, "add -symbols") {
		t.Errorf("-symbol-namespaces without -symbols: exit %d\n%s", code, out)
	}
	out, code = runIn(t, bin, dir, "inspect", "-symbols", "-symbol-namespaces=a,x,!y", "./p")
	if code != 1 || !strings.Contains(out, `no namespace "x", "y"`) || !strings.Contains(out, "\n  c") {
		t.Errorf("unknown -symbol-namespaces: exit %d\n%s", code, out)
	}
}
