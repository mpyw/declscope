package module

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// moduleTree writes a module with one package and returns its root.
func moduleTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.25\n",
		"a/a.go": "package a\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// failingDriver makes every packages.Load fail, while go list -m, which is
// run directly, still answers.
func failingDriver(t *testing.T) {
	t.Helper()
	driver := filepath.Join(t.TempDir(), "driver")
	if err := os.WriteFile(driver, []byte("#!/bin/sh\necho driver refused >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPACKAGESDRIVER", driver)
}

// TestLoadFailures pins that a failure of the go command or of the loader
// stops the run with its error, rather than reading as a module with nothing
// in it.
func TestLoadFailures(t *testing.T) {
	root := moduleTree(t)
	t.Run("the loader fails in Resolve", func(t *testing.T) {
		failingDriver(t)
		if _, err := Resolve(root, nil); err == nil || !strings.Contains(err.Error(), "driver refused") {
			t.Errorf("err = %v, want the driver's failure", err)
		}
	})
	t.Run("the loader fails in Load", func(t *testing.T) {
		failingDriver(t)
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "driver refused") {
			t.Errorf("err = %v, want the driver's failure", err)
		}
	})
	t.Run("go list -m fails", func(t *testing.T) {
		t.Setenv("GOFLAGS", "-no-such-flag")
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "finding the module:") {
			t.Errorf("err = %v, want the go command's failure", err)
		}
	})
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	// failGOWORK fails go env GOWORK when the test cond holds, and runs the
	// real go command otherwise.
	failGOWORK := func(t *testing.T, cond string) {
		fakeGo(t, `if [ "$#" = 2 ] && [ "$1" = env ] && [ "$2" = GOWORK ] && `+cond+`; then echo refused >&2; exit 1; fi; exec `+realGo+` "$@"`)
	}
	t.Run("go env GOWORK fails", func(t *testing.T) {
		failGOWORK(t, "true")
		if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "finding the go.work: ") {
			t.Errorf("err = %v, want the go command's failure", err)
		}
	})
	t.Run("go env GOWORK fails in a nested module", func(t *testing.T) {
		nested := nestedTree(t, map[string]string{"tools/go.mod": "module example.com/m/tools\n\ngo 1.25\n"})
		tools, err := filepath.EvalSymlinks(filepath.Join(nested, "tools"))
		if err != nil {
			t.Fatal(err)
		}
		failGOWORK(t, `[ "$(pwd -P)" = "`+tools+`" ]`)
		m, err := Load(nested)
		if err != nil {
			t.Fatal(err)
		}
		if _, why := m.Range("example.com/m/internal/a"); !strings.Contains(why, "the nested module example.com/m/tools may import it, and finding its go.work failed: ") {
			t.Errorf("why = %q, want the go command's failure", why)
		}
	})
}

// TestWalkFailures pins that the walk stops on a main module it cannot read,
// since a file it skips could hold the one use that keeps a name exported.
func TestWalkFailures(t *testing.T) {
	t.Run("no go.mod", func(t *testing.T) {
		m := &Module{mains: []mainModule{{path: "example.com/m", dir: t.TempDir()}}}
		if err := m.walk(); err == nil {
			t.Error("a main module without go.mod was walked")
		}
	})
	t.Run("an unreadable directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root reads every directory")
		}
		root := moduleTree(t)
		locked := filepath.Join(root, "locked")
		if err := os.Mkdir(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		m := &Module{mains: []mainModule{{path: "example.com/m", dir: root}}}
		if err := m.walk(); err == nil {
			t.Error("a directory the walk could not read was skipped")
		}
	})
}

// TestIgnorePatternsMatch pins the go command's reading of an ignore
// directive: ./x names x at the root only, x names a directory x anywhere,
// and both compare whole path elements.
func TestIgnorePatternsMatch(t *testing.T) {
	ps := ignorePatterns{fromRoot: []string{slashed("gen")}, anywhere: []string{slashed("static"), slashed("web/assets")}}
	for rel, want := range map[string]bool{
		"gen":                 true,
		"gen/sub":             true,
		"internal/gen":        false,
		"generated":           false,
		"static":              true,
		"web/static":          true,
		"web/static/css":      true,
		"nostatic":            false,
		"static2":             false,
		"web/assets":          true,
		"site/web/assets/img": true,
		"web":                 false,
	} {
		if got := ps.matches(rel); got != want {
			t.Errorf("matches(%q) = %v, want %v", rel, got, want)
		}
	}
}

// TestIgnoreDirectivesRefuses pins that a go.mod the walk cannot read stops
// the run rather than reading every ignored directory.
func TestIgnoreDirectivesRefuses(t *testing.T) {
	missing := t.TempDir()
	if _, err := ignoreDirectives(missing); err == nil {
		t.Error("a directory with no go.mod gave no error")
	}
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "go.mod"), []byte("module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ignoreDirectives(broken); err == nil {
		t.Error("a go.mod that does not parse gave no error")
	}
}

// TestLoadReadsNestedModuleAgainstTheMainFileSet pins what makes a nested
// module's uses match the main load's declarations. Its load shares the main
// file set, so keyOf reads its positions where they were written, and the
// main module's packages it imports are roots, read from source. Checking
// the reports alone cannot pin the file set: two loads of a small module
// may happen to add its files in the same order.
func TestLoadReadsNestedModuleAgainstTheMainFileSet(t *testing.T) {
	root, err := filepath.Abs("../testdata/nested-module-uses")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Importers) != 1 {
		t.Fatalf("importers = %d, want the one nested module", len(m.Importers))
	}
	imp := m.Importers[0]
	if len(imp.Pkgs) == 0 {
		t.Fatal("the nested module's own packages were not loaded")
	}
	for _, p := range slices.Concat(imp.Pkgs, imp.Imported) {
		if p.Fset != m.Fset {
			t.Errorf("%s was loaded into a file set of its own", p.ID)
		}
	}
	if len(imp.Imported) != 1 || imp.Imported[0].PkgPath != "example.com/nu/internal/a" || len(imp.Imported[0].Syntax) == 0 {
		t.Errorf("imported = %v, want example.com/nu/internal/a, read from source", imp.Imported)
	}
	if _, why := m.Range("example.com/nu/internal/a"); why != "" {
		t.Errorf("the range is still unjudged: %s", why)
	}
}

// nestedTree writes a module example.com/m with one internal package, and a
// nested module example.com/m/tools holding files, and returns the root.
func nestedTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{
		"go.mod":          "module example.com/m\n\ngo 1.25\n",
		"internal/a/a.go": "package a\n\nfunc F() {}\n",
	}
	for name, body := range files {
		all[name] = body
	}
	for name, body := range all {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestLoadLeavesUnreadableNestedModulesUnjudged pins that a nested module
// this run cannot read keeps the range unjudged, and says why, rather than
// reading as a module that uses nothing.
func TestLoadLeavesUnreadableNestedModulesUnjudged(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		want  string
	}{
		"a go.mod the go command rejects": {
			files: map[string]string{"tools/go.mod": "module example.com/m/tools\n\ngo 1.25\n\nrequire (\n"},
			want:  "loading it failed",
		},
		"a build-excluded file that does not parse": {
			files: map[string]string{
				"tools/go.mod": "module example.com/m/tools\n\ngo 1.25\n",
				"tools/x.go":   "//go:build never\n\npackage tools\n\nthis is not Go\n",
			},
			want: "which the build excludes",
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, err := Load(nestedTree(t, tc.files))
			if err != nil {
				t.Fatal(err)
			}
			_, why := m.Range("example.com/m/internal/a")
			if !strings.Contains(why, "the nested module example.com/m/tools may import it, and ") || !strings.Contains(why, tc.want) {
				t.Errorf("why = %q, want the nested module named and %q", why, tc.want)
			}
		})
	}
}

// TestLoadReadsNestedWorkspace pins which copy of a module that a nested
// module's own go.work uses from outside the tree withholds the fixes in the
// range. It is a main module of that build, not of this run. A copy whose
// files, under any build constraint, import an internal package of the range
// does. One that imports none cannot break the build, nor can a file that
// does not parse. One that cannot be read counts as if it did. The range is
// judged either way.
func TestLoadReadsNestedWorkspace(t *testing.T) {
	const prefix = "the build of the nested module example.com/m/tools uses example.com/m/ext from "
	for name, tc := range map[string]struct {
		files map[string]string
		// locked is a directory of the copy made unreadable.
		locked string
		want   func(ext string) string
	}{
		"imports an internal package under a tag": {
			files: map[string]string{"e.go": "//go:build never\n\npackage ext\n\nimport \"example.com/m/internal/a\"\n\nfunc Run() { a.F() }\n"},
			want:  func(ext string) string { return prefix + ext + ", which imports example.com/m/internal/a" },
		},
		"imports none": {
			files: map[string]string{
				"e.go":                "package ext\n\nimport \"example.com/m/pub\"\n\nvar _ = pub.X\n",
				"sub/go.mod":          "module example.com/m/ext/sub\n",
				"sub/s.go":            "package sub\n\nimport \"example.com/m/internal/a\"\n",
				"vendor/v/v.go":       "package v\n\nimport \"example.com/m/internal/a\"\n",
				"README.md":           "not Go\n",
				"internal/own/own.go": "package own\n",
			},
			want: func(string) string { return "" },
		},
		// A package of the copy may import any of these, though ./... skips
		// them.
		"imports from testdata": {
			files: map[string]string{"testdata/t/t.go": "package t\n\nimport \"example.com/m/internal/a\"\n"},
			want:  func(ext string) string { return prefix + ext + ", which imports example.com/m/internal/a" },
		},
		"imports from a directory starting with _": {
			files: map[string]string{"_impl/i.go": "package impl\n\nimport \"example.com/m/internal/a\"\n"},
			want:  func(ext string) string { return prefix + ext + ", which imports example.com/m/internal/a" },
		},
		"imports from a directory starting with .": {
			files: map[string]string{".gen/g.go": "package gen\n\nimport \"example.com/m/internal/a\"\n"},
			want:  func(ext string) string { return prefix + ext + ", which imports example.com/m/internal/a" },
		},
		"holds a file that does not parse": {
			files: map[string]string{
				"e.go":            "this is not Go\n",
				"testdata/bad.go": "package bad\n\nimport \"example.com/m/internal/a\n",
			},
			want: func(string) string { return "" },
		},
		"cannot be read": {
			files:  map[string]string{"e.go": "package ext\n", "locked/l.go": "package locked\n"},
			locked: "locked",
			want:   func(ext string) string { return prefix + ext },
		},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.locked != "" && os.Getuid() == 0 {
				t.Skip("root reads every directory")
			}
			ext, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			tc.files["go.mod"] = "module example.com/m/ext\n\ngo 1.25\n"
			for name, body := range tc.files {
				path := filepath.Join(ext, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.locked != "" {
				locked := filepath.Join(ext, tc.locked)
				if err := os.Chmod(locked, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
			}
			m, err := Load(nestedTree(t, map[string]string{
				"tools/go.mod":  "module example.com/m/tools\n\ngo 1.25\n",
				"tools/go.work": "go 1.25\n\nuse (\n\t.\n\t..\n\t" + ext + "\n)\n",
			}))
			if err != nil {
				t.Fatal(err)
			}
			if why := m.Doubt("example.com/m/internal/a"); why != tc.want(ext) {
				t.Errorf("doubt = %q, want %q", why, tc.want(ext))
			}
			if _, why := m.Range("example.com/m/internal/a"); why != "" {
				t.Errorf("why = %q, want the range judged", why)
			}
		})
	}
}

// TestListUncovered pins what the build list of a nested module that no
// internal parent covers says about the fixes. k cannot import
// example.com/m/s/internal/a, so it is not loaded, but a module it requires
// may.
func TestListUncovered(t *testing.T) {
	for name, tc := range map[string]struct {
		mod, want string
	}{
		"a go.mod that does not parse builds nothing": {
			mod: "module example.com/m/k\n\nrequire (\n",
		},
		"a pruned go.mod passes over a module outside the main modules": {
			mod: "module example.com/m/k\n\ngo 1.21\n\nrequire example.org/gone v1.0.0\n",
		},
		"a pruned go.mod keeps a module in the range it cannot find": {
			mod:  "module example.com/m/k\n\ngo 1.21\n\nrequire example.com/m/s/gone v1.0.0\n",
			want: "the build of the nested module example.com/m/k uses example.com/m/s/gone from example.com/m/s/gone@v1.0.0",
		},
		"a go.mod before 1.17 may hide what it requires": {
			mod:  "module example.com/m/k\n\ngo 1.16\n\nrequire example.org/gone v1.0.0\n",
			want: "the build list of the nested module example.com/m/k cannot be read, and a module it names may use it: reading the build list failed: example.org/gone: ",
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for name, body := range map[string]string{
				"go.mod":            "module example.com/m\n\ngo 1.25\n",
				"s/internal/a/a.go": "package a\n\nfunc F() {}\n",
				"k/go.mod":          tc.mod,
			} {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			m, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if why := m.Doubt("example.com/m/s/internal/a"); !strings.HasPrefix(why, tc.want) || (tc.want == "") != (why == "") {
				t.Errorf("doubt = %q, want %q", why, tc.want)
			}
			if _, why := m.Range("example.com/m/s/internal/a"); why != "" {
				t.Errorf("why = %q, want the range judged", why)
			}
		})
	}
	t.Run("a go.mod that cannot be read", func(t *testing.T) {
		if l := (&Module{}).listUncovered(nestedModule{dir: t.TempDir()}); l.unbuilt == "" {
			t.Error("unbuilt is empty, want the read error")
		}
	})
	t.Run("go env GOWORK fails", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m/k\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fakeGo(t, "echo refused >&2; exit 1")
		if l := (&Module{}).listUncovered(nestedModule{dir: dir}); !strings.HasPrefix(l.unbuilt, "finding its go.work failed: ") {
			t.Errorf("unbuilt = %q, want the go command's failure", l.unbuilt)
		}
	})
}

// TestUntidyNestedGoMod pins that a nested go.mod that lacks what its build
// needs withholds every fix. Here d raises t to v1.1.0, which k's go.mod does
// not say. go list -m -e all then quietly lists t v1.1.0, a clean copy, while
// the build compiles v1.0.0, which uses foo.Hello.
func TestUntidyNestedGoMod(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"m/go.mod":            "module example.com/m\n\ngo 1.22\n",
		"m/internal/foo/f.go": "package foo\n\nfunc Hello() string { return \"hi\" }\n",
		"t1/go.mod":           "module example.com/m/t\n\ngo 1.22\n",
		"t1/t.go":             "package t\n\nimport \"example.com/m/internal/foo\"\n\nvar V = foo.Hello()\n",
		"t2/go.mod":           "module example.com/m/t\n\ngo 1.22\n",
		"t2/t.go":             "package t\n\nvar V = \"clean\"\n",
		"d/go.mod":            "module other.local/d\n\ngo 1.22\n\nrequire example.com/m/t v1.1.0\n",
		"d/d.go":              "package d\n",
		"m/k/go.mod": "module other.local/k\n\ngo 1.22\n\nrequire (\n\texample.com/m v0.0.0\n\texample.com/m/t v1.0.0\n\tother.local/d v1.0.0\n)\n\n" +
			"replace (\n\texample.com/m => ../\n\texample.com/m/t v1.0.0 => ../../t1\n\texample.com/m/t v1.1.0 => ../../t2\n\tother.local/d v1.0.0 => ../../d\n)\n",
		"m/k/main.go": "package main\n\nimport \"example.com/m/t\"\n\nfunc main() { println(t.V) }\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := Load(filepath.Join(root, "m"))
	if err != nil {
		t.Fatal(err)
	}
	want := "the build list of the nested module other.local/k cannot be read, and a module it names may use it: its go.mod needs updates"
	if why := m.Doubt("example.com/m/internal/foo"); !strings.HasPrefix(why, want) {
		t.Errorf("doubt = %q, want it to start with %q", why, want)
	}
}

// TestHolds pins which module may hold a package in the tree at
// example.com/m/x/y: one whose path lies inside the tree, and one whose path
// lies above it whose directory has y outside any nested module, or whose
// directory is unknown.
func TestHolds(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"has/y/z.go", "nested/y/go.mod", "deep/y/sub/go.mod"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const parent = "example.com/m/x/y"
	for _, tc := range []struct {
		name, path, dir string
		want            bool
	}{
		{"the tree itself", "example.com/m/x/y", "", true},
		{"inside the tree", "example.com/m/x/y/z", filepath.Join(dir, "missing"), true},
		{"unrelated", "example.com/m/w", filepath.Join(dir, "has"), false},
		{"above, with no directory", "example.com/m/x", "", true},
		{"above, holding the tree's directory", "example.com/m/x", filepath.Join(dir, "has"), true},
		{"above, without the tree's directory", "example.com/m/x", filepath.Join(dir, "missing"), false},
		{"above, the directory a nested module", "example.com/m/x", filepath.Join(dir, "nested"), false},
		{"above, the directory holding a nested module", "example.com/m/x", filepath.Join(dir, "deep"), true},
	} {
		if got := holds(tc.path, tc.dir, parent); got != tc.want {
			t.Errorf("%s: holds(%q, %q) = %v, want %v", tc.name, tc.path, tc.dir, got, tc.want)
		}
	}
	t.Run("above, the directory cannot be read", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root reads every directory")
		}
		locked := filepath.Join(dir, "locked")
		if err := os.MkdirAll(filepath.Join(locked, "y"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		if !holds("example.com/m/x", locked, parent) {
			t.Error("holds = false, want true: nothing shows the directory is not there")
		}
	})
}

// TestBuildListAboveTheMainModule pins that a module of the main module's
// build list whose path lies above the main module's leaves the range
// unjudged when its copy holds the range's directory: example.com, read from
// ../out, holds example.com/m/q, which imports example.com/m/internal/foo.
func TestBuildListAboveTheMainModule(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"m/go.mod":            "module example.com/m\n\ngo 1.24\n\nrequire example.com v0.0.0\n\nreplace example.com => ../out\n",
		"m/internal/foo/f.go": "package foo\n\nfunc Hello() string { return \"hi\" }\n",
		"m/cmd/app/main.go":   "package main\n\nimport \"example.com/m/q\"\n\nfunc main() { println(q.V) }\n",
		"out/go.mod":          "module example.com\n\ngo 1.24\n",
		"out/m/q/q.go":        "package q\n\nimport \"example.com/m/internal/foo\"\n\nvar V = foo.Hello()\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := Load(filepath.Join(root, "m"))
	if err != nil {
		t.Fatal(err)
	}
	want := "the build uses example.com from " + filepath.Join(root, "out")
	if _, why := m.Range("example.com/m/internal/foo"); why != want {
		t.Errorf("why = %q, want %q", why, want)
	}
}

// TestCopiesOutsideTheRange pins that a copy only reaches the ranges it may
// import into: one never scanned counts in a range holding its path alone, a
// package outside internal/ has no range to doubt, and a copy that cannot be
// walked is never scanned.
func TestCopiesOutsideTheRange(t *testing.T) {
	m := &Module{
		mains: []mainModule{{path: "example.com/m"}},
		built: []builtModule{{path: "example.com/m/b/x", by: "example.com/m/b/n"}},
	}
	for _, path := range []string{"example.com/m/a/internal/p", "example.com/m/b/x/pub"} {
		if why := m.Doubt(path); why != "" {
			t.Errorf("%s: doubt = %q, want none", path, why)
		}
	}
	if _, err := importsUnder(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("err = nil, want the walk's failure")
	}
}

// TestBuildListFailures pins that a build list this run cannot read in full
// leaves every range unjudged with the reason, rather than reading as one
// that names no module in the range.
func TestBuildListFailures(t *testing.T) {
	root := nestedTree(t, map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.25\n\nrequire example.com/m/gone v1.0.0\n",
	})
	mains := []mainModule{{path: "example.com/m", dir: root}}
	t.Run("a module the go command cannot read", func(t *testing.T) {
		t.Setenv("GOPROXY", "off")
		t.Setenv("GOFLAGS", "-mod=mod")
		if _, why := buildList(root, root, nil, mains, false); !strings.Contains(why, "reading the build list failed: ") || !strings.Contains(why, "example.com/m/gone") {
			t.Errorf("why = %q, want the module that failed named", why)
		}
	})
	missing := filepath.Join(root, "missing")
	t.Run("go list -m all fails", func(t *testing.T) {
		if _, why := buildList(missing, root, nil, mains, false); !strings.HasPrefix(why, "reading the build list failed: ") {
			t.Errorf("why = %q, want the go command's failure", why)
		}
	})
	inAny := func(string) bool { return true }
	t.Run("go env fails in vendor mode", func(t *testing.T) {
		if _, why := vendored(missing, root, nil, inAny, inAny); !strings.HasPrefix(why, "reading the build list failed: ") {
			t.Errorf("why = %q, want the go command's failure", why)
		}
	})
	t.Run("modules.txt cannot be read", func(t *testing.T) {
		if err := os.MkdirAll(filepath.Join(root, "vendor", "modules.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, why := vendored(root, root, nil, inAny, inAny); !strings.HasPrefix(why, "reading the build list failed: ") {
			t.Errorf("why = %q, want the read error", why)
		}
	})
	t.Run("go list -m prints what is not JSON", func(t *testing.T) {
		fakeGo(t, "echo '{'")
		if _, why := buildList(root, root, nil, mains, false); !strings.HasPrefix(why, "reading the build list failed: ") {
			t.Errorf("why = %q, want the decode error", why)
		}
	})
	t.Run("a nested module's", func(t *testing.T) {
		m, err := Load(nestedTree(t, map[string]string{"tools/go.mod": "module example.com/m/tools\n\ngo 1.25\n\nrequire (\n"}))
		if err != nil {
			t.Fatal(err)
		}
		// Every fix is withheld, and no range is unjudged for it.
		if m.unbuilt != "" || !strings.HasPrefix(m.Doubt("example.com/m/internal/a"), "the build list of the nested module example.com/m/tools cannot be read, and a module it names may use it: reading the build list failed: ") {
			t.Errorf("unbuilt = %q, doubt = %q, want every fix withheld naming the nested module", m.unbuilt, m.Doubt("example.com/m/internal/a"))
		}
	})
	t.Run("Range names it", func(t *testing.T) {
		m := &Module{mains: mains, unbuilt: "reading the build list failed: x"}
		if _, why := m.Range("example.com/m/internal/a"); why != m.unbuilt {
			t.Errorf("why = %q, want %q", why, m.unbuilt)
		}
	})
}

// TestBuildListVendored pins where vendor mode reads the modules from:
// vendor/modules.txt of the workspace when there is one, else of the main
// module, and nothing when there is none.
func TestBuildListVendored(t *testing.T) {
	work := t.TempDir()
	main := filepath.Join(work, "m")
	mains := []mainModule{{path: "example.com/m", dir: main}}
	vendorErr := `if [ "$1" = list ]; then echo "go: can't compute 'all' using the vendor directory" >&2; exit 1; fi; `
	t.Run("no modules.txt", func(t *testing.T) {
		fakeGo(t, vendorErr+"echo")
		if built, why := buildList(work, main, nil, mains, false); built != nil || why != "" {
			t.Errorf("built = %+v, why = %q, want nothing", built, why)
		}
	})
	t.Run("the workspace's modules.txt", func(t *testing.T) {
		fakeGo(t, vendorErr+"echo "+filepath.Join(work, "go.work"))
		txt := "## workspace\n# example.com/m\n# example.com/m/x v1.0.0\n## explicit; go 1.25\nexample.com/m/x\n# example.com/other v1.0.0\n"
		if err := os.MkdirAll(filepath.Join(work, "vendor"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, "vendor", "modules.txt"), []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
		built, why := buildList(work, main, nil, mains, false)
		want := []builtModule{{path: "example.com/m/x", dir: filepath.Join(work, "vendor", "example.com", "m", "x")}}
		if !reflect.DeepEqual(built, want) || why != "" {
			t.Errorf("built = %+v, why = %q, want %+v", built, why, want)
		}
	})
}

// fakeGo puts a go command first on PATH that runs script under sh.
func fakeGo(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(filepath.ListSeparator)+os.Getenv("PATH"))
}

// TestReaches pins which import of a build-excluded file may bring a value of
// a package's types into it. A package outside the modules this run reads
// cannot import an internal package of theirs. A package of those modules
// that the run did not load may: it can import one that hands the value out.
// That holds for a nested module whose path is unrelated too, since it may
// import the main module's own API.
func TestReaches(t *testing.T) {
	a := &packages.Package{PkgPath: "example.com/m/internal/a"}
	api := &packages.Package{PkgPath: "example.com/m/api", Imports: map[string]*packages.Package{a.PkgPath: a}}
	other := &packages.Package{PkgPath: "example.com/m/other"}
	m := &Module{
		mains:  []mainModule{{path: "example.com/m"}},
		nested: []nestedModule{{path: "example.com/m/tools"}, {path: "example.com/other"}},
		Pkgs:   []*packages.Package{a, api, other},
	}
	for from, want := range map[string]bool{
		"example.com/m/internal/a": true,  // the package itself
		"example.com/m/api":        true,  // loaded, and imports it
		"example.com/m/other":      false, // loaded, and does not
		"example.com/m/gone":       true,  // a main module's, not loaded
		"example.com/m/tools/x":    true,  // a nested module's, not loaded
		"example.com/other/x":      true,  // a nested module's with an unrelated path
		"fmt":                      false,
		"example.com/elsewhere":    false,
	} {
		if got := m.Reaches(from, a.PkgPath); got != want {
			t.Errorf("Reaches(%q) = %v, want %v", from, got, want)
		}
	}
}

// TestIsTestMain pins what marks the synthetic executable of a test binary.
// The go command hands it back either as its generated _testmain.go or as a
// build-cache archive. A real main package whose path ends in ".test" is
// loaded from Go source and is kept. A package with no file shows nothing, so
// it is not called one.
func TestIsTestMain(t *testing.T) {
	for _, tc := range []struct {
		name string
		pkg  *packages.Package
		want bool
	}{
		{"generated _testmain.go", &packages.Package{Name: "main", PkgPath: "example.com/m/a.test", GoFiles: []string{"/cache/b001/_testmain.go"}}, true},
		{"_testmain.go only among compiled files", &packages.Package{Name: "main", PkgPath: "example.com/m/a.test", CompiledGoFiles: []string{"/cache/b001/_testmain.go"}}, true},
		{"build-cache archive", &packages.Package{Name: "main", PkgPath: "example.com/m/a.test", GoFiles: []string{"/cache/0a/0a1b2c-d"}}, true},
		{"real package ending in .test", &packages.Package{Name: "main", PkgPath: "example.com/m/cmd.test", GoFiles: []string{"/src/cmd.test/main.go"}}, false},
		{"no files", &packages.Package{Name: "main", PkgPath: "example.com/m/a.test"}, false},
		{"not named main", &packages.Package{Name: "a", PkgPath: "example.com/m/a.test", GoFiles: []string{"/cache/b001/_testmain.go"}}, false},
		{"path not ending in .test", &packages.Package{Name: "main", PkgPath: "example.com/m/cmd", GoFiles: []string{"/cache/b001/_testmain.go"}}, false},
	} {
		if got := IsTestMain(tc.pkg); got != tc.want {
			t.Errorf("%s: IsTestMain = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestErrorLine pins how a failed package is named: by one line, with an
// absolute position where the error has one, and with no "-" position where it
// has none. The go command's build output is passed over for the type error
// that follows it, except where that error only says C could not be imported.
func TestErrorLine(t *testing.T) {
	build := func(lines ...string) packages.Error {
		return packages.Error{Pos: "-", Msg: strings.Join(append([]string{"# example.com/m/a"}, lines...), "\n"), Kind: packages.ListError}
	}
	typeError := packages.Error{Pos: "/m/a/a.go:3:14", Msg: `cannot use "s" (untyped string constant) as int value in variable declaration`, Kind: packages.TypeError}
	noC := packages.Error{Pos: "/m/a/a.go:4:8", Msg: "could not import C (no metadata for C)", Kind: packages.TypeError}
	for _, tc := range []struct {
		name   string
		errors []packages.Error
		want   string
	}{
		{"the type error after the build output", []packages.Error{build(`a/a.go:3:14: cannot use "s"`), typeError}, `/m/a/a.go:3:14: cannot use "s" (untyped string constant) as int value in variable declaration`},
		{"the build output alone", []packages.Error{build(`a/a.go:3:14: cannot use "s"`, "a/a.go:4:1: too many errors")}, `/m/a/a.go:3:14: cannot use "s"`},
		{"the C error over could not import C", []packages.Error{build("a/a.go: In function 'f':", "a/a.go:3:22: error: 'undefined_c' undeclared"), noC}, "/m/a/a.go:3:22: error: 'undefined_c' undeclared"},
		{"could not import C alone", []packages.Error{noC}, "/m/a/a.go:4:8: could not import C (no metadata for C)"},
		{"a go command error with no position", []packages.Error{{Pos: "", Msg: "import cycle not allowed: a imports b\n\timports a", Kind: packages.ListError}}, "import cycle not allowed: a imports b \timports a"},
	} {
		if got := ErrorLine(&packages.Package{Errors: tc.errors}, "/m"); got != tc.want {
			t.Errorf("%s: ErrorLine = %q, want %q", tc.name, got, tc.want)
		}
	}
}
