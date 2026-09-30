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

// TestShrinkFixFailure pins that -fix stops with the error when it cannot
// write a file, rather than exiting as though the fix were applied.
func TestShrinkFixFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes every file")
	}
	root := shrinkModule(t)
	if err := os.Chmod(filepath.Join(root, "internal/a/a.go"), 0o444); err != nil {
		t.Fatal(err)
	}
	out, code := runIn(t, bin, root, "shrink", "-fix")
	if code != 1 || !strings.Contains(out, "declscope shrink:") || !strings.Contains(out, "permission denied") {
		t.Fatalf("exit %d, want a refusal naming the write error:\n%s", code, out)
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
	// fmt resolves without a module, so the refusal comes from looking for
	// one. go list -m answers command-line-arguments there, with no directory.
	out, code = runIn(t, bin, dir, "shrink", "fmt")
	if code != 1 || !strings.Contains(out, "declscope shrink: not inside a module") {
		t.Fatalf("exit %d, want a refusal naming the missing module:\n%s", code, out)
	}
}

func TestShrinkUsage(t *testing.T) {
	out, code := runIn(t, bin, t.TempDir(), "shrink", "-h")
	if code != 0 || !strings.Contains(out, "Usage: declscope shrink") || !strings.Contains(out, "-fix") {
		t.Fatalf("exit %d, want the usage and the flags:\n%s", code, out)
	}
}

// TestShrinkWorkspace pins that a workspace is shrunk as one: every module
// of it is loaded, so a nested module the workspace uses is an importer this
// run sees, not one it must stand down for.
func TestShrinkWorkspace(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/root\n\ngo 1.25\n")
	writeTree(t, root, "internal/a/a.go", "package a\n\nfunc Used() {}\n\nfunc Unused() {}\n")
	writeTree(t, root, "tool/go.mod", "module example.com/root/tool\n\ngo 1.25\n\nrequire example.com/root v0.0.0\n")
	writeTree(t, root, "tool/main.go", "package main\n\nimport \"example.com/root/internal/a\"\n\nfunc main() { a.Used() }\n")
	writeTree(t, root, "other/go.mod", "module example.com/other\n\ngo 1.25\n")
	writeTree(t, root, "other/internal/o/o.go", "package o\n\nfunc Lone() {}\n")

	// Without the workspace, tool is a nested module this run does not load.
	out, code := runIn(t, bin, root, "shrink")
	if code != 0 || !strings.Contains(out, "the nested module example.com/root/tool may import it") {
		t.Fatalf("exit %d, want internal/a unjudged over the nested module:\n%s", code, out)
	}

	writeTree(t, root, "go.work", "go 1.25\n\nuse (\n\t.\n\t./tool\n\t./other\n)\n")
	out, code = runIn(t, bin, root, "shrink", "./...", "./other/...")
	if code != 3 {
		t.Fatalf("exit %d, want 3 for a report:\n%s", code, out)
	}
	for _, want := range []string{
		"func Unused is exported, but nothing outside example.com/root/internal/a uses it\n",
		"func Lone is exported, but nothing outside example.com/other/internal/o uses it\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Used is exported") || strings.Contains(out, "not judged") {
		t.Errorf("tool uses a.Used and is loaded with the workspace:\n%s", out)
	}
}

// TestShrinkPatternErrors pins that a pattern the go command rejects stops
// the run with the go command's message, as go vet does. Passing it silently
// would let a typo in CI check nothing.
func TestShrinkPatternErrors(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, "sub/go.mod", "module example.com/sub\n\ngo 1.25\n")
	writeTree(t, root, "sub/s.go", "package s\n")
	for pattern, want := range map[string]string{
		"./nope/...": "pattern ./nope/...: lstat ./nope/: no such file or directory",
		"./sub/...":  "pattern ./sub/...: directory prefix sub does not contain main module",
	} {
		out, code := runIn(t, bin, root, "shrink", pattern)
		if code != 1 || !strings.Contains(out, "declscope shrink: "+want) {
			t.Errorf("%s: exit %d, want 1 with %q:\n%s", pattern, code, want, out)
		}
	}
}

// TestShrinkUnmatchedPatterns pins go vet's handling of a pattern that
// matches no package: a warning while another pattern matches, and a refusal
// once none does.
func TestShrinkUnmatchedPatterns(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, "empty/README", "no Go here\n")
	out, code := runIn(t, bin, root, "shrink", "./empty/...", "./internal/...")
	if code != 3 || !strings.Contains(out, `declscope shrink: warning: "./empty/..." matched no packages`) || !strings.Contains(out, "func Lonely is exported") {
		t.Fatalf("exit %d, want a warning and the report:\n%s", code, out)
	}
	out, code = runIn(t, bin, root, "shrink", "./empty/...")
	if code != 1 || !strings.Contains(out, "declscope shrink: ./empty/... matched no packages") {
		t.Fatalf("exit %d, want a refusal:\n%s", code, out)
	}
}

// TestShrinkNamesWhatItDoesNotLoad pins that an internal package ./...
// leaves out, named on its own, is reported as not judged: the go command
// resolves it, but the load never read it or its uses.
func TestShrinkNamesWhatItDoesNotLoad(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, "internal/a/testdata/fixture/f.go", "package fixture\n\nfunc Lone() {}\n")
	out, code := runIn(t, bin, root, "shrink", "./internal/a/testdata/fixture")
	want := "declscope shrink: not judged: example.com/declscopetest/internal/a/testdata/fixture: ./... leaves it out, so this run does not load it\n"
	if code != 0 || !strings.Contains(out, want) {
		t.Fatalf("exit %d, want %q:\n%s", code, want, out)
	}
}

// TestShrinkCountsPackagesOutside pins that a pattern naming packages outside
// the main module is followed, and that they are counted as not judged in one
// line rather than listed or dropped.
func TestShrinkCountsPackagesOutside(t *testing.T) {
	root := shrinkModule(t)
	out, code := runIn(t, bin, root, "shrink", "fmt", "errors", "./...")
	if code != 3 || !strings.Contains(out, "declscope shrink: not judged: 2 package(s) outside the main module\n") || !strings.Contains(out, "func Lonely is exported") {
		t.Fatalf("exit %d, want the report and one line counting fmt and errors:\n%s", code, out)
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
	if code != 0 || !strings.Contains(out, "not judged: example.com/x/internal/y:") || strings.Contains(out, "Unused is exported") {
		t.Fatalf("exit %d, want nothing judged, and the package named as such:\n%s", code, out)
	}
}

// TestShrinkSkipsWhatGoSkips pins that the walk for build-excluded files
// skips what ./... skips. A testdata file naming a declaration is no use.
func TestShrinkSkipsWhatGoSkips(t *testing.T) {
	root := shrinkModule(t)
	// ignore directives, one for a directory at the root only and one for a
	// directory of that name anywhere, as go help go.mod describes them.
	writeTree(t, root, "go.mod", testModule+"\nignore (\n\t./gen\n\tstatic\n)\n")
	for _, dir := range []string{
		"internal/a/testdata", "internal/a/.hidden", "internal/a/_skipped", "vendor/example.com/v",
		// ./... skips a vendor directory at any depth, and nothing may import
		// a path through one.
		"internal/a/vendor/w",
		"gen", "web/static",
	} {
		writeTree(t, root, dir+"/x.go", "//go:build never\n\npackage x\n\nimport \"example.com/declscopetest/internal/a\"\n\nvar _ = a.Lonely\n")
	}
	// An ignored directory is not read at all, so a file there that does not
	// parse refuses nothing, as it does not for go vet.
	writeTree(t, root, "gen/broken.go", "package gen\n\nthis is not Go\n")
	// ./gen names the root's gen only, so this one is read.
	writeTree(t, root, "internal/gen/x.go", "//go:build never\n\npackage gen\n\nimport \"example.com/declscopetest/internal/a\"\n\nvar _ = a.Taken\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 3 || !strings.Contains(out, "func Lonely is exported, but nothing outside") {
		t.Fatalf("exit %d, want Lonely still reported:\n%s", code, out)
	}
	if strings.Contains(out, "Taken") {
		t.Fatalf("a.Taken is named by a build-excluded file outside every ignored directory, but was reported:\n%s", out)
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

// TestShrinkRefusesNestedModuleWithoutPath pins that a nested go.mod naming
// no module refuses the run: whether it may import an internal package
// depends on that path.
func TestShrinkRefusesNestedModuleWithoutPath(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, "tools/go.mod", "go 1.25\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 1 || !strings.Contains(out, "no module path") {
		t.Fatalf("exit %d, want a refusal naming the go.mod:\n%s", code, out)
	}
}

// TestShrinkNamesSkippedPackages pins that an internal package left
// unjudged is named on stderr with the reason, while the exit status still
// says nothing was reported.
func TestShrinkNamesSkippedPackages(t *testing.T) {
	root := shrinkModule(t)
	// A nested module that imports internal/a but does not type-check, so
	// whether it uses anything there is unknown.
	writeTree(t, root, "tools/go.mod", "module example.com/declscopetest/tools\n\ngo 1.25\n\nrequire example.com/declscopetest v0.0.0\n\nreplace example.com/declscopetest => ../\n")
	writeTree(t, root, "tools/t.go", "package tools\n\nimport \"example.com/declscopetest/internal/a\"\n\nvar _ string = a.Used()\n")
	cmd := exec.Command(bin, "shrink")
	cmd.Dir = root
	coverEnv(cmd)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil || len(out) != 0 {
		t.Fatalf("err %v, want a silent success on stdout:\n%s", err, out)
	}
	if !strings.Contains(stderr.String(), "not judged: example.com/declscopetest/internal/a: the nested module example.com/declscopetest/tools may import it, and its package example.com/declscopetest/tools does not type-check") {
		t.Errorf("stderr does not name the skipped package:\n%s", stderr.String())
	}
}

// TestShrinkSkipsGitAndAcceptsMissingExamples pins two things a module can
// hold that ./... never builds. The walk does not enter .git, so a Go file
// there is no excluded file. An example function naming nothing is left
// alone, though go vet would reject it.
func TestShrinkSkipsGitAndAcceptsMissingExamples(t *testing.T) {
	root := shrinkModule(t)
	writeTree(t, root, ".git/x.go", "package x\n\nfunc {\n")
	writeTree(t, root, "internal/a/a_test.go", "package a\n\nfunc ExampleMissing() {}\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 3 || !strings.Contains(out, "func Lonely is exported, but nothing outside") {
		t.Fatalf("exit %d, want Lonely reported and nothing refused:\n%s", code, out)
	}
}

func TestShrinkRefusesEmptyModule(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", testModule)
	out, code := runIn(t, bin, root, "shrink")
	// ./... names nothing there, so the run stops where go vet does.
	if code != 1 || !strings.Contains(out, "matched no packages") {
		t.Fatalf("exit %d, want a refusal for a module with no package:\n%s", code, out)
	}
	// fmt names a package, just none of the module's.
	out, code = runIn(t, bin, root, "shrink", "fmt")
	if code != 1 || !strings.Contains(out, "no package found in the module") {
		t.Fatalf("exit %d, want a refusal for a module with no package:\n%s", code, out)
	}
}

// TestShrinkSkipsCgo pins that a package using cgo is named as not judged.
// It needs a C toolchain, so it is skipped where cgo is off.
func TestShrinkSkipsCgo(t *testing.T) {
	if out, err := exec.Command("go", "env", "CGO_ENABLED").Output(); err != nil || strings.TrimSpace(string(out)) != "1" {
		t.Skip("cgo is not enabled")
	}
	root := shrinkModule(t)
	writeTree(t, root, "internal/c/c.go", "package c\n\n// int one(void) { return 1; }\nimport \"C\"\n\nfunc One() int { return int(C.one()) }\n")
	out, code := runIn(t, bin, root, "shrink")
	if code != 3 || !strings.Contains(out, "not judged: example.com/declscopetest/internal/c: the package uses cgo") || strings.Contains(out, "func One is") {
		t.Fatalf("exit %d, want internal/c named as not judged for cgo:\n%s", code, out)
	}
}
