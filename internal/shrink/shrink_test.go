package shrink_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal/shrink"
)

// Each module under testdata marks what it expects with a comment on the
// line of the report: // want: <regexp>, matched against the message. Every
// report must match a want on its line, and every want must be matched, the
// way analysistest checks a package.

var wantComment = regexp.MustCompile(`// want: (.*)$`)

// shrinkWants reads every want comment of the module, keyed by file:line.
func shrinkWants(t *testing.T, dir string) map[string]*regexp.Regexp {
	t.Helper()
	wants := map[string]*regexp.Regexp{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, text := range strings.Split(string(src), "\n") {
			if m := wantComment.FindStringSubmatch(text); m != nil {
				wants[path+":"+strconv.Itoa(i+1)] = regexp.MustCompile(m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return wants
}

func shrinkCheck(t *testing.T, dir string) []shrink.Finding {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	res, err := shrink.Run(abs, nil)
	if err != nil {
		t.Fatal(err)
	}
	findings := res.Findings
	wants := shrinkWants(t, abs)
	for _, f := range findings {
		at := f.Pos.Filename + ":" + strconv.Itoa(f.Pos.Line)
		re, ok := wants[at]
		if !ok {
			t.Errorf("%s: unexpected report: %s", at, f.Message())
			continue
		}
		if !re.MatchString(f.Message()) {
			t.Errorf("%s: report %q does not match %q", at, f.Message(), re)
		}
		delete(wants, at)
	}
	for at, re := range wants {
		t.Errorf("%s: no report matching %q", at, re)
	}
	return findings
}

// Each module under testdata is named for what it pins. The modules that
// keep a whole range unjudged are separate, since one nested module there
// silences every package it may import.

// TestShrinkUses pins what counts as a use from outside the package: a
// spelled name, an unkeyed literal, a struct conversion, a linkname, a static
// satisfaction, a value handed out by an importable package, a type an API
// carries, an external test, and a build-excluded file.
func TestShrinkUses(t *testing.T) { shrinkCheck(t, "testdata/uses") }

// TestShrinkEscapes pins that a value escaping into an interface is a use.
func TestShrinkEscapes(t *testing.T) { shrinkCheck(t, "testdata/escapes") }

// TestShrinkFixWithheld pins the reports whose fix is withheld for a reason
// other than the rename: build-excluded and generated files, example
// functions, and -ldflags -X.
func TestShrinkFixWithheld(t *testing.T) { shrinkCheck(t, "testdata/fix-withheld") }

// TestShrinkRenameGuards pins what the rename rewrites and every guard that
// withholds it.
func TestShrinkRenameGuards(t *testing.T) { shrinkCheck(t, "testdata/rename-guards") }

// TestShrinkIgnores pins where //declscope:ignore overexported binds, and
// when it is itself reported unused.
func TestShrinkIgnores(t *testing.T) { shrinkCheck(t, "testdata/ignores") }

// TestShrinkNotJudged pins the packages that are never judged, and that the
// internal ones among them are named with the reason.
func TestShrinkNotJudged(t *testing.T) {
	shrinkCheck(t, "testdata/not-judged")
	dir, err := filepath.Abs("testdata/not-judged")
	if err != nil {
		t.Fatal(err)
	}
	res, err := shrink.Run(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range res.Skipped {
		got[s.Package] = s.Reason
	}
	want := map[string]string{
		"example.com/notjudged/internal/asm":  "the package holds non-Go files",
		"example.com/notjudged/internal/crc":  "the package holds non-Go files",
		"example.com/notjudged/internal/tool": "package main",
	}
	if len(got) != len(want) {
		t.Errorf("skipped = %v, want %v: a package outside internal/ is not listed", got, want)
	}
	for pkg, reason := range want {
		if got[pkg] != reason {
			t.Errorf("%s skipped for %q, want %q", pkg, got[pkg], reason)
		}
	}
}

// TestShrinkNestedModule pins that a nested module whose path extends an
// internal/ parent keeps every package under it unjudged.
func TestShrinkNestedModule(t *testing.T) { shrinkCheck(t, "testdata/nested-module") }

// TestShrinkNestedModuleInUnderscoreDir pins that a nested module in a
// directory ./... skips is still found and loaded, so the use it makes of
// a.F counts and nothing is left unjudged.
func TestShrinkNestedModuleInUnderscoreDir(t *testing.T) {
	res := shrinkRun(t, "testdata/nested-module-in-underscore-dir")
	if len(res.Skipped) != 0 {
		t.Errorf("skipped = %+v, want none: the nested module is loaded", res.Skipped)
	}
}

// TestShrinkNestedModuleUses pins that a loaded nested module counts as an
// importer in every way the evidence reads: a call, a static interface
// satisfaction, a value that escapes into fmt, and a name written in a file
// its build excludes.
func TestShrinkNestedModuleUses(t *testing.T) {
	res := shrinkRun(t, "testdata/nested-module-uses")
	if len(res.Skipped) != 0 {
		t.Errorf("skipped = %+v, want none", res.Skipped)
	}
}

// TestShrinkNestedModuleElsewhere pins that a nested module reading the
// module from another directory leaves the range unjudged, and says where
// it reads it from.
func TestShrinkNestedModuleElsewhere(t *testing.T) {
	res := shrinkRun(t, "testdata/nested-module-elsewhere")
	if len(res.Skipped) != 1 || res.Skipped[0].Package != "example.com/ne/internal/a" ||
		!strings.Contains(res.Skipped[0].Reason, "the nested module example.com/ne/tools may import it, and it reads example.com/ne/internal/a from") ||
		!strings.Contains(res.Skipped[0].Reason, "nested-module-elsewhere-copy") {
		t.Errorf("skipped = %+v, want internal/a, naming the copy it is read from", res.Skipped)
	}
}

// TestShrinkNestedModuleBroken pins that a nested module that does not
// type-check leaves the range unjudged, rather than reading as a module that
// uses nothing.
func TestShrinkNestedModuleBroken(t *testing.T) {
	res := shrinkRun(t, "testdata/nested-module-broken")
	if len(res.Skipped) != 1 || res.Skipped[0].Package != "example.com/nb/internal/a" ||
		!strings.Contains(res.Skipped[0].Reason, "does not type-check") {
		t.Errorf("skipped = %+v, want internal/a, naming the type error", res.Skipped)
	}
}

// shrinkRun checks a module's wants, as shrinkCheck does, and returns the
// whole result.
func shrinkRun(t *testing.T, dir string) shrink.Result {
	t.Helper()
	shrinkCheck(t, dir)
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	res, err := shrink.Run(abs, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestShrinkNestedModuleInTestdata pins the same for a nested module one
// level below a directory ./... skips.
func TestShrinkNestedModuleInTestdata(t *testing.T) {
	shrinkCheck(t, "testdata/nested-module-in-testdata")
}

// TestShrinkModuleUnderInternal pins that a package whose path runs through
// internal/ is still an exposure root when a module this run does not load
// may import it.
func TestShrinkModuleUnderInternal(t *testing.T) { shrinkCheck(t, "testdata/module-under-internal") }

// TestShrinkRenamesDoc pins what the fix does to a doc comment: the name it
// opens with is rewritten, after an article too, and another word is not.
func TestShrinkRenamesDoc(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/rename-guards")); err != nil {
		t.Fatal(err)
	}
	dir, _ = filepath.EvalSymlinks(dir)
	res, err := shrink.Run(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := shrink.Apply(res.Findings); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(dir, "internal/r/r.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"// plain is fixed, and the name its doc comment opens with is rewritten too.\nfunc plain()",
		"// A thing is fixed, and so is the name after the article.\ntype thing struct{}",
		"// serve is fixed with its doc comment.\nfunc (Svc) serve()",
		"// Loader opens this comment, which is another word, and is left alone.\nfunc load()",
	} {
		want = strings.ReplaceAll(want, "(Svc)", "(svc)")
		if !strings.Contains(string(src), want) {
			t.Errorf("after the fix, missing:\n%s", want)
		}
	}
}

// TestShrinkConverges applies every fix in one pass to a copy of the module,
// and checks the three things a fix owes: the module still passes go vet,
// which also compiles the tests; every report that offered a fix is gone; and
// no report appears that was not there before.
func TestShrinkConverges(t *testing.T) {
	for _, name := range []string{"uses", "escapes", "fix-withheld", "rename-guards", "ignores", "module-under-internal", "nested-module-uses"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.CopyFS(dir, os.DirFS(filepath.Join("testdata", name))); err != nil {
				t.Fatal(err)
			}
			dir, _ = filepath.EvalSymlinks(dir)
			res, err := shrink.Run(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			before := res.Findings
			fixed := 0
			kept := map[string]bool{}
			for _, f := range before {
				if f.Fix != nil {
					fixed++
					continue
				}
				kept[f.Pos.Filename+":"+strconv.Itoa(f.Pos.Line)+" "+f.Message()] = true
			}
			if fixed == 0 {
				t.Fatal("no fix offered, so nothing converges")
			}
			if err := shrink.Apply(before); err != nil {
				t.Fatal(err)
			}
			// composites is off: uses writes an unkeyed literal of another
			// package's struct on purpose, since that is a use to count.
			// Every module of the copy, nested ones included: a fix must not
			// break a nested module that imports the one fixed.
			err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || d.Name() != "go.mod" {
					return err
				}
				vet := exec.Command("go", "vet", "-composites=false", "./...")
				vet.Dir = filepath.Dir(path)
				if out, err := vet.CombinedOutput(); err != nil {
					t.Fatalf("go vet in %s after the fix: %v\n%s", vet.Dir, err, out)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			res, err = shrink.Run(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			after := res.Findings
			for _, f := range after {
				at := f.Pos.Filename + ":" + strconv.Itoa(f.Pos.Line) + " " + f.Message()
				if !kept[at] {
					t.Errorf("after the fix: %s", at)
				}
				delete(kept, at)
			}
			for at := range kept {
				t.Errorf("a report without a fix disappeared: %s", at)
			}
		})
	}
}

// TestApplyFailures pins that a fix that cannot be written stops with the
// error, file by file, rather than leaving the reader to think it was
// applied.
func TestApplyFailures(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads and writes every file")
	}
	dir := t.TempDir()
	edit := func(name string) []shrink.Finding {
		return []shrink.Finding{{Fix: []shrink.Edit{{Filename: filepath.Join(dir, name), Start: 0, End: 1, NewText: "x"}}}}
	}
	for name, mode := range map[string]os.FileMode{"unreadable.go": 0o200, "readonly.go": 0o444} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package p\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"missing.go", "unreadable.go", "readonly.go"} {
		if err := shrink.Apply(edit(name)); err == nil {
			t.Errorf("%s: the fix was reported applied", name)
		}
	}
}
