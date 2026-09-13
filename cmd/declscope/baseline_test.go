package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal/baseline"
)

// These run the built binary against a temporary module, because the
// properties under test are about where the subcommand writes and what the
// analyzer then finds — a unit test of either half proves nothing about the
// handshake between them. Each case ends by running the analyzer, since a
// baseline is only correct if its presence suppresses what it records.

// twoFiles is a package with a boundary and a qualify violation on helper,
// referenced from the ordinary and the test variant alike so that the two
// hand over the same key twice.
var twoFiles = map[string]string{
	"user.go":       "package x\n\nfunc helper() int { return 1 }\n",
	"order.go":      "package x\n\nfunc orderRun() int { return helper() }\n\nvar _ = orderRun\n",
	"order_test.go": "package x\n\nimport \"testing\"\n\nfunc TestOrder(t *testing.T) { _ = helper() }\n",
}

func TestBaselinePerPackageTargets(t *testing.T) {
	// A config in sub/ names its own baseline, and deep/ carries its own
	// default-named one; each shadows the root for the packages under it.
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/m\n\ngo 1.25\n")
	writeTree(t, root, "sub/.declscope.yaml", "baseline: sub-baseline.yaml\n")
	writeTree(t, root, "deep/.declscope-baseline.yaml", "")
	for _, pkg := range []string{"", "sub/inner", "deep/inner", "plain"} {
		for name, body := range twoFiles {
			writeTree(t, root, filepath.Join(pkg, name), body)
		}
	}

	out, code := runIn(t, bin, root, "baseline", "./...")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}

	want := map[string][]string{
		".declscope-baseline.yaml":      {"example.com/m", "example.com/m/plain"},
		"sub/sub-baseline.yaml":         {"example.com/m/sub/inner"},
		"deep/.declscope-baseline.yaml": {"example.com/m/deep/inner"},
	}
	for file, pkgs := range want {
		set := load(t, filepath.Join(root, file))
		for _, pkg := range pkgs {
			if !set.Has(baseline.Key{Package: pkg, Rule: "boundary", Namespace: "user", Decl: "helper"}) {
				t.Errorf("%s lacks the entry for %s:\n%s", file, pkg, out)
			}
		}
		if set.Len() != 2*len(pkgs) {
			t.Errorf("%s holds %d entries, want %d: entries of other packages leaked in", file, set.Len(), 2*len(pkgs))
		}
		if !strings.Contains(out, "in "+filepath.Join(root, file)) {
			t.Errorf("the report does not name %s:\n%s", file, out)
		}
	}

	assertSuppressed(t, bin, root)
}

// TestBaselineFromSubdirectory is the data-loss case: a run from store/ must
// write under store/ and leave the root baseline — which holds entries for
// packages the run never saw — exactly as it was.
func TestBaselineFromSubdirectory(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/m\n\ngo 1.25\n")
	for _, pkg := range []string{"", "store/inner"} {
		for name, body := range twoFiles {
			writeTree(t, root, filepath.Join(pkg, name), body)
		}
	}
	if _, code := runIn(t, bin, root, "baseline", "./..."); code != 0 {
		t.Fatal("generating the root baseline")
	}
	before, err := os.ReadFile(filepath.Join(root, ".declscope-baseline.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	store := filepath.Join(root, "store")
	out, code := runIn(t, bin, store, "baseline", "./...")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}

	after, err := os.ReadFile(filepath.Join(root, ".declscope-baseline.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("a run from store/ rewrote the root baseline:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	set := load(t, filepath.Join(store, ".declscope-baseline.yaml"))
	if !set.Has(baseline.Key{Package: "example.com/m/store/inner", Rule: "boundary", Namespace: "user", Decl: "helper"}) {
		t.Errorf("store/.declscope-baseline.yaml should hold store's entries:\n%s", out)
	}
	if !strings.Contains(out, "in "+filepath.Join(store, ".declscope-baseline.yaml")) {
		t.Errorf("the report should name the file under store/:\n%s", out)
	}

	assertSuppressed(t, bin, root)
}

// TestBaselineRefusesUnreachableDefault covers the one case where writing in
// the working directory would be a lie: a package whose lookup stops at its
// own go.mod before reaching it. A workspace is what makes such a package
// addressable from the root at all. The root's own package is placeable, and
// still nothing is written for it: a refusal is all or nothing, so a partial
// run cannot leave a baseline that looks complete.
func TestBaselineRefusesUnreachableDefault(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/m\n\ngo 1.25\n")
	writeTree(t, root, "go.work", "go 1.25\n\nuse (\n\t.\n\t./nested\n)\n")
	writeTree(t, root, "nested/go.mod", "module example.com/nested\n\ngo 1.25\n")
	for _, pkg := range []string{"", "nested"} {
		for name, body := range twoFiles {
			writeTree(t, root, filepath.Join(pkg, name), body)
		}
	}

	out, code := runIn(t, bin, root, "baseline", "./...", "./nested/...")
	if code == 0 {
		t.Fatalf("a baseline nobody would find must not be written silently:\n%s", out)
	}
	if !strings.Contains(out, "example.com/nested") {
		t.Errorf("the refusal should name the package:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".declscope-baseline.yaml")); err == nil {
		t.Error("nothing should have been written, not even for the placeable package")
	}

	// With -o the caller has taken over placement, so the run goes through
	// and both packages land in the one file.
	out, code = runIn(t, bin, root, "baseline", "-o", "all.yaml", "./...", "./nested/...")
	if code != 0 {
		t.Fatalf("-o should place everything in one file: exit %d\n%s", code, out)
	}
	set := load(t, filepath.Join(root, "all.yaml"))
	for _, pkg := range []string{"example.com/m", "example.com/nested"} {
		if !set.Has(baseline.Key{Package: pkg, Rule: "boundary", Namespace: "user", Decl: "helper"}) {
			t.Errorf("all.yaml lacks the entry for %s:\n%s", pkg, out)
		}
	}
}

func TestBaselineRegeneratesCorruptFile(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/m\n\ngo 1.25\n")
	for name, body := range twoFiles {
		writeTree(t, root, name, body)
	}
	corrupt := "packages:\n  x:\n    boundary:\n      user: [helper]\nbogus: 1\n"
	writeTree(t, root, ".declscope-baseline.yaml", corrupt)

	// The analyzer must refuse the file, otherwise a typo would silently
	// suppress nothing...
	if out, code := runIn(t, bin, root, "./..."); code == 0 || !strings.Contains(out, "bogus") {
		t.Fatalf("the analyzer should reject a corrupt baseline: exit %d\n%s", code, out)
	}
	// ...but regenerating, which is what the file itself says to do, must
	// not be blocked by it.
	out, code := runIn(t, bin, root, "baseline", "./...")
	if code != 0 {
		t.Fatalf("a corrupt baseline blocked its own regeneration: exit %d\n%s", code, out)
	}
	set := load(t, filepath.Join(root, ".declscope-baseline.yaml"))
	if !set.Has(baseline.Key{Package: "example.com/m", Rule: "boundary", Namespace: "user", Decl: "helper"}) {
		t.Error("the regenerated file should hold the current violations")
	}
	assertSuppressed(t, bin, root)

	// The same with -o: a corrupt file that is not even the target must not
	// get in the way.
	writeTree(t, root, ".declscope-baseline.yaml", corrupt)
	if out, code := runIn(t, bin, root, "baseline", "-o", "alt.yaml", "./..."); code != 0 {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

// TestBaselineReportsWrittenCount checks the number in "recorded N
// violation(s)" against the file: the ordinary and the test variant of a
// package hand over the same keys, and only one copy is written.
func TestBaselineReportsWrittenCount(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/m\n\ngo 1.25\n")
	for name, body := range twoFiles {
		writeTree(t, root, name, body)
	}

	out, code := runIn(t, bin, root, "baseline", "./...")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	m := regexp.MustCompile(`recorded (\d+) violation`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no count reported:\n%s", out)
	}
	reported, _ := strconv.Atoi(m[1])
	data, err := os.ReadFile(filepath.Join(root, ".declscope-baseline.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	inFile := len(entryLine.FindAllString(string(data), -1))
	if reported != inFile || inFile != 2 {
		t.Errorf("reported %d, file holds %d, want 2 (boundary and qualify on helper):\n%s", reported, inFile, data)
	}
}

// assertSuppressed runs the analyzer over the module from root and expects
// silence: every violation the baseline recorded must be found by the lookup
// from its own package.
func assertSuppressed(t *testing.T, bin, root string) {
	t.Helper()
	if out, code := runIn(t, bin, root, "./..."); code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("the generated baseline does not suppress what it recorded: exit %d\n%s", code, out)
	}
}

func load(t *testing.T, path string) *baseline.Set {
	t.Helper()
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return set
}

// runIn executes the binary in dir and returns its combined output and exit
// code. Diagnostics and refusals both exit non-zero, so only a failure to
// start is fatal.
func runIn(t *testing.T, bin, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running declscope %v: %v\n%s", args, err, out)
		}
		return string(out), exit.ExitCode()
	}
	return string(out), 0
}

// bin is the linter under test, built once per process: every test here
// drives the same binary, and a build per test multiplies the memory that a
// full `go test ./...` peaks at.
var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "declscope-baseline-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin = filepath.Join(dir, "declscope")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building the linter: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func writeTree(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// entryLine matches one recorded declaration, at whatever depth the file
// nests: counting a fixed indent would quietly read zero if the shape changed.
var entryLine = regexp.MustCompile(`(?m)^ +- \S`)

// TestBaselineDoesNotSurviveAMove checks that an entry stops matching once the
// declaration moves to a file in another namespace.
//
// boundary is a statement about which namespaces a use crosses. The same
// declaration reached from the same file is a different violation once it is
// declared somewhere else, and a key blind to that would go on suppressing a
// crossing nobody recorded.
func TestBaselineDoesNotSurviveAMove(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "go.mod", "module example.com/m\n\ngo 1.25\n")
	// The naming rules would otherwise report helper in both placements and
	// drown the one thing under test.
	writeTree(t, root, ".declscope.yaml", "rules:\n  qualify: never\n")
	writeTree(t, root, "user.go", "package x\n\nfunc helper() int { return 1 }\n")
	writeTree(t, root, "csv.go", "package x\n\nfunc csvRun() int { return helper() }\n\nvar _ = csvRun\n")

	if out, code := runIn(t, bin, root, "baseline", "./..."); code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	assertSuppressed(t, bin, root)

	// The same declaration and the same use site, in a different namespace.
	if err := os.Remove(filepath.Join(root, "user.go")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, "order.go", "package x\n\nfunc helper() int { return 1 }\n")

	out, code := runIn(t, bin, root, "./...")
	if code == 0 {
		t.Fatalf("the crossing is now order -> csv, which the baseline never recorded:\n%s", out)
	}
	if !strings.Contains(out, `is private to namespace "order"`) {
		t.Errorf("want the report to name the namespace it crosses now, got:\n%s", out)
	}
}
