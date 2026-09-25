package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These drive the built binary, like the other subcommands' tests: what they
// assert is the handshake between the command line, the loaded module and the
// files -fix writes. internal/shrink's own tests cover what is reported.

// shrinkModule is a module with one declaration nothing outside its package
// uses, one another package uses, and one whose fix is withheld.
func shrinkModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, "go.mod", testModule)
	writeTree(t, root, "internal/a/a.go", "package a\n\n"+
		"func Lonely() int { return 1 }\n\n"+
		"func Used() int { return Lonely() + Taken() }\n\n"+
		"func Taken() int { return taken() }\n\n"+
		"func taken() int { return 0 }\n")
	writeTree(t, root, "b/b.go", "package b\n\nimport \"example.com/declscopetest/internal/a\"\n\nvar _ = a.Used()\n")
	return root
}

func TestShrinkReports(t *testing.T) {
	root := shrinkModule(t)
	out, code := runIn(t, bin, root, "shrink")
	if code != 3 {
		t.Fatalf("exit %d, want 3 for a report:\n%s", code, out)
	}
	for _, want := range []string{
		"func Lonely is exported, but nothing outside example.com/declscopetest/internal/a uses it\n",
		"func Taken is exported, but nothing outside example.com/declscopetest/internal/a uses it (no fix: the unexported name is taken or would be captured)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Used") {
		t.Errorf("Used is named by package b, but was reported:\n%s", out)
	}
}

func TestShrinkFix(t *testing.T) {
	root := shrinkModule(t)
	out, code := runIn(t, bin, root, "shrink", "-fix")
	// The withheld report is still printed, so the run still reports.
	if code != 3 || strings.Contains(out, "Lonely") || !strings.Contains(out, "Taken") {
		t.Fatalf("exit %d, want 3 with only the withheld report left:\n%s", code, out)
	}
	src, err := os.ReadFile(filepath.Join(root, "internal/a/a.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "func lonely() int") || !strings.Contains(string(src), "return lonely() + Taken()") {
		t.Errorf("the fix did not rename every identifier:\n%s", src)
	}
	vet := exec.Command("go", "vet", "./...")
	vet.Dir = root
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("go vet after the fix: %v\n%s", err, out)
	}
}

// TestShrinkPatterns pins that the patterns choose what is reported, not
// what is loaded: package b still counts as a user of a.Used.
func TestShrinkPatterns(t *testing.T) {
	root := shrinkModule(t)
	out, code := runIn(t, bin, filepath.Join(root, "b"), "shrink", ".")
	if code != 0 || out != "" {
		t.Fatalf("exit %d, want a silent run for a package with nothing to report:\n%s", code, out)
	}
	out, code = runIn(t, bin, filepath.Join(root, "internal/a"), "shrink", ".")
	if code != 3 || strings.Contains(out, "Used") {
		t.Fatalf("exit %d, want a.Used counted as used by b though b is not named:\n%s", code, out)
	}
}

// TestShrinkRefusesTypeErrors pins that a module that does not type-check is
// refused: a package with no references reads exactly like nothing using a
// declaration.
func TestShrinkRefusesTypeErrors(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, "c/c.go", "package c\n\nvar _ int = \"x\"\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 1 || !strings.Contains(out, "do not type-check") {
		t.Fatalf("exit %d, want a refusal naming the type error:\n%s", code, out)
	}
}

func TestShrinkRefusesOutsideAModule(t *testing.T) {
	dir := t.TempDir()
	out, code := runIn(t, bin, dir, "shrink")
	if code != 1 || !strings.Contains(out, "declscope shrink:") {
		t.Fatalf("exit %d, want a refusal:\n%s", code, out)
	}
}

func TestShrinkUsage(t *testing.T) {
	out, code := runIn(t, bin, t.TempDir(), "shrink", "-h")
	if code != 0 || !strings.Contains(out, "Usage: declscope shrink") || !strings.Contains(out, "-fix") {
		t.Fatalf("exit %d, want the usage and the flags:\n%s", code, out)
	}
}

// TestShrinkRefusesWorkspace pins that a workspace is refused: another of
// its modules may import this one's internal packages, and which one to
// shrink would be a guess.
func TestShrinkRefusesWorkspace(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "a/go.mod", "module example.com/a\n\ngo 1.25\n")
	writeTree(t, root, "a/a.go", "package a\n")
	writeTree(t, root, "b/go.mod", "module example.com/b\n\ngo 1.25\n")
	writeTree(t, root, "b/b.go", "package b\n")
	writeTree(t, root, "go.work", "go 1.25\n\nuse (\n\t./a\n\t./b\n)\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 1 || !strings.Contains(out, "GOWORK=off") {
		t.Fatalf("exit %d, want a refusal pointing at GOWORK=off:\n%s", code, out)
	}
}

// TestShrinkModuleUnderInternal pins that a module whose own path runs
// through internal/ judges nothing there: the tree allowed to import it lies
// above the module, where this run loads nothing.
func TestShrinkModuleUnderInternal(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/x/internal/y\n\ngo 1.25\n")
	writeTree(t, root, "y.go", "package y\n\nfunc Unused() {}\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 0 || out != "" {
		t.Fatalf("exit %d, want nothing judged:\n%s", code, out)
	}
}

// TestShrinkSkipsWhatGoSkips pins that the walk for build-excluded files
// skips what ./... skips. A testdata file naming a declaration is no use.
func TestShrinkSkipsWhatGoSkips(t *testing.T) {
	root := shrinkModule(t)
	for _, dir := range []string{"internal/a/testdata", "internal/a/.hidden", "internal/a/_skipped", "vendor/example.com/v"} {
		writeTree(t, root, dir+"/x.go", "//go:build never\n\npackage x\n\nimport \"example.com/declscopetest/internal/a\"\n\nvar _ = a.Lonely\n")
	}
	// A vendor directory below the root holds an ordinary package, and its
	// build-excluded file is read like any other.
	writeTree(t, root, "internal/a/vendor/w/w.go", "package w\n")
	writeTree(t, root, "internal/a/vendor/w/w_never.go", "//go:build never\n\npackage w\n\nimport \"example.com/declscopetest/internal/a\"\n\nvar _ = a.Taken\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 3 || !strings.Contains(out, "func Lonely is exported, but nothing outside") {
		t.Fatalf("exit %d, want Lonely still reported:\n%s", code, out)
	}
	if strings.Contains(out, "Taken") {
		t.Fatalf("a.Taken is named by a build-excluded file under a nested vendor directory, but was reported:\n%s", out)
	}
}

// TestShrinkRefusesUnreadableExcludedFile pins that a build-excluded file
// that does not parse refuses the run: it may name anything.
func TestShrinkRefusesUnreadableExcludedFile(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, "b/b_never.go", "//go:build never\n\npackage b\n\nfunc {\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 1 || !strings.Contains(out, "which the build excludes") {
		t.Fatalf("exit %d, want a refusal naming the file:\n%s", code, out)
	}
}
