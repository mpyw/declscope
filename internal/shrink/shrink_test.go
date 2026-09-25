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
	findings, err := shrink.Run(abs, nil)
	if err != nil {
		t.Fatal(err)
	}
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

func TestShrinkBasic(t *testing.T) { shrinkCheck(t, "testdata/basic") }

func TestShrinkEdge(t *testing.T) { shrinkCheck(t, "testdata/edge") }

func TestShrinkMore(t *testing.T) { shrinkCheck(t, "testdata/more") }

// TestShrinkReview holds one case per finding of the review of the first
// version, each of which the first version got wrong.
func TestShrinkReview(t *testing.T) { shrinkCheck(t, "testdata/review") }

// TestShrinkOuter pins that a package whose path runs through internal/ is
// still an exposure root when a module this run does not load may import it.
func TestShrinkOuter(t *testing.T) { shrinkCheck(t, "testdata/outer") }

// TestShrinkTools pins that a nested module in a directory ./... skips still
// keeps the packages it may import unjudged.
func TestShrinkTools(t *testing.T) { shrinkCheck(t, "testdata/tools") }

// TestShrinkNested pins that a nested module under an internal parent keeps
// every package under it unjudged: the module could import them, and this run
// never loads it.
func TestShrinkNested(t *testing.T) { shrinkCheck(t, "testdata/nested") }

// TestShrinkConverges applies every fix in one pass to a copy of the module,
// and checks the three things a fix owes: the module still passes go vet,
// which also compiles the tests; every report that offered a fix is gone; and
// no report appears that was not there before.
func TestShrinkConverges(t *testing.T) {
	for _, name := range []string{"basic", "edge", "more", "review", "outer"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.CopyFS(dir, os.DirFS(filepath.Join("testdata", name))); err != nil {
				t.Fatal(err)
			}
			dir, _ = filepath.EvalSymlinks(dir)
			before, err := shrink.Run(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
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
			// composites is off: edge writes an unkeyed literal of another
			// package's struct on purpose, since that is a use to count.
			vet := exec.Command("go", "vet", "-composites=false", "./...")
			vet.Dir = dir
			if out, err := vet.CombinedOutput(); err != nil {
				t.Fatalf("go vet after the fix: %v\n%s", err, out)
			}
			after, err := shrink.Run(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
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
