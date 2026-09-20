package main_test

import (
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
	if !strings.Contains(out, "Package   example.com/declscopetest/q\n") {
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
