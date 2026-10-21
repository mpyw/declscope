package buildtag

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

func TestConstrained(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"user.go", "package p\n", false},

		// A GOOS or GOARCH suffix, as go/build reads it.
		{"run_linux.go", "package p\n", true},
		{"run_amd64.go", "package p\n", true},
		{"run_linux_amd64.go", "package p\n", true},
		{"run_linux_test.go", "package p\n", true},
		{"a/b/run_windows.go", "package p\n", true},

		// Only the part after the first underscore is read.
		{"linux.go", "package p\n", false},
		{"linux_test.go", "package p\n", false},
		{"amd64_linux.go", "package p\n", true},
		{"run_linuxish.go", "package p\n", false},

		// Only the part before the first dot is read.
		{"run_linux.pb.go", "package p\n", true},
		{"run.x_linux.go", "package p\n", false},
		{"x.linux_amd64.go", "package p\n", false},

		// mips8 is not a GOARCH go/build knows.
		{"run_mips8.go", "package p\n", false},

		// A constraint line before the package clause.
		{"gui.go", "//go:build production || dev\n\npackage p\n", true},
		{"gui.go", "// +build production\n\npackage p\n", true},
		{"gui.go", "// Package p is doc.\n//go:build !never\n\npackage p\n", true},

		// After the package clause, the go command reads none.
		{"gui.go", "package p\n\n//go:build never\n", false},
		{"gui.go", "// go:build never\n\npackage p\n", false},
	}
	for _, tt := range tests {
		f, err := parser.ParseFile(token.NewFileSet(), tt.name, tt.src, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if got := Constrained(tt.name, f); got != tt.want {
			t.Errorf("Constrained(%q, %q) = %v, want %v", tt.name, tt.src, got, tt.want)
		}
	}
}

func TestTrimOSArch(t *testing.T) {
	tests := []struct {
		stem string
		want string
		ok   bool
	}{
		{"run", "run", false},
		{"run_linux", "run", true},
		{"run_linux_amd64", "run", true},
		{"run_linux_amd64_test", "run_test", true},
		{"run_amd64_linux", "run_amd64", true},
		{"linux_amd64", "linux", true},
		{"linux", "linux", false},
		{"linux_test", "linux_test", false},
		{"run_test", "run_test", false},
		{"run_mips8", "run_mips8", false},
	}
	for _, tt := range tests {
		if got, ok := TrimOSArch(tt.stem); got != tt.want || ok != tt.ok {
			t.Errorf("TrimOSArch(%q) = %q, %v, want %q, %v", tt.stem, got, ok, tt.want, tt.ok)
		}
	}
}

// TestKnownLists checks knownOS and knownArch against the lists go/build
// reads file names with, so that a Go release that changes them fails here.
func TestKnownLists(t *testing.T) {
	path := filepath.Join(build.Default.GOROOT, "src", "internal", "syslist", "syslist.go")
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for name, ours := range map[string]map[string]bool{"KnownOS": knownOS, "KnownArch": knownArch} {
		want := syslistKeys(t, f, name)
		if got := slices.Sorted(maps.Keys(ours)); !slices.Equal(got, want) {
			t.Errorf("%s:\n got %q\nwant %q", name, got, want)
		}
	}
}

// syslistKeys returns the sorted keys that the map literal assigned to the
// package-level variable name sets to true.
func syslistKeys(t *testing.T, f *ast.File, name string) []string {
	t.Helper()
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, spec := range g.Specs {
			vs := spec.(*ast.ValueSpec)
			if len(vs.Names) != 1 || vs.Names[0].Name != name || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatalf("%s is not a composite literal", name)
			}
			var keys []string
			for _, elt := range lit.Elts {
				kv := elt.(*ast.KeyValueExpr)
				key, err := strconv.Unquote(kv.Key.(*ast.BasicLit).Value)
				if err != nil {
					t.Fatal(err)
				}
				if v, ok := kv.Value.(*ast.Ident); ok && v.Name == "true" {
					keys = append(keys, key)
				}
			}
			slices.Sort(keys)
			return keys
		}
	}
	t.Fatalf("no %s in syslist", name)
	return nil
}

// TestConstrainedMatchesGoBuild compares the file-name part of Constrained
// with build.Context.MatchFile. Under a GOOS and GOARCH that no suffix names,
// MatchFile leaves out exactly the files whose name constrains them.
func TestConstrainedMatchesGoBuild(t *testing.T) {
	names := []string{
		"foo.go", "foo_test.go", "linux.go", "linux_test.go", "amd64_linux.go",
		"linux_amd64.go", "foo_linuxish.go", "foo_amd64_linux.go",
		"foo_linux_linux.go", "foo_linux_test_test.go", "foo_x_test_amd64.go",
		"foo__linux.go", "foo_linux.go.go", "foo_linux.pb.go", "foo.bar_linux.go",
		"x.linux_amd64.go", "foo_mips8.go", "foo_test.pb.go",
		"foo_linux_test.pb.go", "foo.bar_test.go",
	}
	for goos := range knownOS {
		names = append(names, "f_"+goos+".go", "f_"+goos+"_test.go", "f_"+goos+".x.go", "f.x_"+goos+".go")
		for goarch := range knownArch {
			names = append(names, "f_"+goos+"_"+goarch+".go", "f_"+goos+"_"+goarch+"_test.go")
		}
	}
	for goarch := range knownArch {
		names = append(names, "f_"+goarch+".go", "f_"+goarch+"_test.go", "f_"+goarch+".x.go", "f.x_"+goarch+".go")
	}

	dir := t.TempDir()
	ctx := build.Default
	ctx.GOOS, ctx.GOARCH = "noos", "noarch"
	ctx.BuildTags, ctx.ToolTags, ctx.ReleaseTags = nil, nil, nil
	src := "package p\n"
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		match, err := ctx.MatchFile(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if got := Constrained(name, f); got != !match {
			t.Errorf("Constrained(%q) = %v, but MatchFile = %v", name, got, match)
		}
	}
}
