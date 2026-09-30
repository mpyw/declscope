package module

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
