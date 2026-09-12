package directive_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/scope"
)

func parse(t *testing.T, src string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func firstFunc(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	for _, d := range parse(t, src).Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatal("no func declaration")
	return nil
}

func TestParseDeclScope(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    scope.Scope
		wantHas bool
	}{
		{"package", "//declscope:package", scope.PackageInternal, true},
		{"file", "//declscope:file", scope.FilePrivate, true},
		{"public", "//declscope:public", scope.Public, true},
		{"spaced", "// declscope:package", scope.PackageInternal, true},
		{"block", "/*declscope:package*/", scope.PackageInternal, true},
		{"with reason", "//declscope:package // shared with the reporter", scope.PackageInternal, true},
		{"unrelated", "// an ordinary comment", 0, false},
		{"other tool", "//nolint:all", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := firstFunc(t, "package p\n\n"+tt.comment+"\nfunc f() {}\n")
			d := directive.ParseDecl(fn.Doc)
			if d.HasScope != tt.wantHas {
				t.Fatalf("HasScope = %v, want %v (problems: %v)", d.HasScope, tt.wantHas, d.Problems)
			}
			if tt.wantHas && d.Scope != tt.want {
				t.Errorf("Scope = %v, want %v", d.Scope, tt.want)
			}
		})
	}
}

func TestParseDeclProblems(t *testing.T) {
	tests := []struct {
		name    string
		comment string
	}{
		{"unknown keyword", "//declscope:bogus"},
		{"argument where none is taken", "//declscope:package user"},
		{"ignore with argument", "//declscope:ignore why"},
		{"namespace on a declaration", "//declscope:namespace user"},
		{"conflicting scopes", "//declscope:package\n//declscope:file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := firstFunc(t, "package p\n\n"+tt.comment+"\nfunc f() {}\n")
			if d := directive.ParseDecl(fn.Doc); len(d.Problems) == 0 {
				t.Errorf("want a problem, got none")
			}
		})
	}
}

func TestParseDeclIgnore(t *testing.T) {
	fn := firstFunc(t, "package p\n\n//declscope:ignore\nfunc f() {}\n")
	if d := directive.ParseDecl(fn.Doc); !d.Ignore {
		t.Error("Ignore = false, want true")
	}
}

func TestMerge(t *testing.T) {
	outer := directive.Decl{Scope: scope.PackageInternal, HasScope: true}
	inner := directive.Decl{Scope: scope.FilePrivate, HasScope: true}

	if got := outer.Merge(inner); got.Scope != scope.FilePrivate {
		t.Errorf("a spec directive should override its block, got %v", got.Scope)
	}
	if got := outer.Merge(directive.Decl{}); got.Scope != scope.PackageInternal {
		t.Errorf("a block directive should survive an empty spec, got %v", got.Scope)
	}
}

func TestFileNamespace(t *testing.T) {
	f := parse(t, "//declscope:namespace user\npackage repo\n\nfunc f() {}\n")
	name, _, ok, problems := directive.FileNamespace(f)
	if !ok || name != "user" {
		t.Fatalf("FileNamespace = %q, %v, want \"user\", true", name, ok)
	}
	if len(problems) != 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
}

func TestFileNamespaceProblems(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"missing name", "//declscope:namespace\npackage repo\n"},
		{"exported name", "//declscope:namespace User\npackage repo\n"},
		{"not an identifier", "//declscope:namespace user-repo\npackage repo\n"},
		{"duplicate", "//declscope:namespace user\n//declscope:namespace order\npackage repo\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, problems := directive.FileNamespace(parse(t, tt.src))
			if len(problems) == 0 {
				t.Error("want a problem, got none")
			}
		})
	}
}

// TestFileNamespaceIgnoresDeclarations checks that a namespace directive after
// the package clause is not mistaken for a file-level one.
func TestFileNamespaceIgnoresDeclarations(t *testing.T) {
	f := parse(t, "package repo\n\n//declscope:namespace user\nfunc f() {}\n")
	if _, _, ok, _ := directive.FileNamespace(f); ok {
		t.Error("a namespace directive below the package clause should not apply to the file")
	}
}
