package directive_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
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
		{"private", "//declscope:private", scope.Private, true},
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
		{"ignore of an unknown rule", "//declscope:ignore why"},
		{"namespace on a declaration", "//declscope:namespace user"},
		{"conflicting scopes", "//declscope:package\n//declscope:private"},
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
	tests := []struct {
		name    string
		comment string
		covers  []rule.Rule
		misses  []rule.Rule
	}{
		{
			name:    "bare ignore covers every rule",
			comment: "//declscope:ignore",
			covers:  rule.All,
		},
		{
			name:    "one rule",
			comment: "//declscope:ignore unqualify",
			covers:  []rule.Rule{rule.Unqualify},
			misses:  []rule.Rule{rule.Boundary, rule.Qualify},
		},
		{
			name:    "several rules",
			comment: "//declscope:ignore unqualify,qualify",
			covers:  []rule.Rule{rule.Unqualify, rule.Qualify},
			misses:  []rule.Rule{rule.Boundary},
		},
		{
			name:    "spaces around the separator",
			comment: "//declscope:ignore unqualify, qualify",
			covers:  []rule.Rule{rule.Unqualify, rule.Qualify},
			misses:  []rule.Rule{rule.Boundary},
		},
		{
			name:    "with a reason",
			comment: "//declscope:ignore unqualify // the prefix is part of the concept",
			covers:  []rule.Rule{rule.Unqualify},
			misses:  []rule.Rule{rule.Boundary},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := firstFunc(t, "package p\n\n"+tt.comment+"\nfunc f() {}\n")
			d := directive.ParseDecl(fn.Doc)
			if len(d.Problems) != 0 {
				t.Fatalf("unexpected problems: %v", d.Problems)
			}
			if len(d.Ignores) != 1 {
				t.Fatalf("got %d ignores, want 1", len(d.Ignores))
			}
			for _, r := range tt.covers {
				if !d.Ignores[0].Covers(r) {
					t.Errorf("Covers(%q) = false, want true", r)
				}
			}
			for _, r := range tt.misses {
				if d.Ignores[0].Covers(r) {
					t.Errorf("Covers(%q) = true, want false", r)
				}
			}
		})
	}
}

// TestParseDeclIgnoreAccumulates checks that several ignore directives on one
// declaration are all kept, so that each can be reported unused on its own.
func TestParseDeclIgnoreAccumulates(t *testing.T) {
	fn := firstFunc(t, "package p\n\n//declscope:ignore unqualify\n//declscope:ignore qualify\nfunc f() {}\n")
	if d := directive.ParseDecl(fn.Doc); len(d.Ignores) != 2 {
		t.Errorf("got %d ignores, want 2", len(d.Ignores))
	}
}

func TestMerge(t *testing.T) {
	outer := directive.Decl{Scope: scope.PackageInternal, HasScope: true}
	inner := directive.Decl{Scope: scope.Private, HasScope: true}

	if got := outer.Merge(inner); got.Scope != scope.Private {
		t.Errorf("a spec directive should override its block, got %v", got.Scope)
	}
	if got := outer.Merge(directive.Decl{}); got.Scope != scope.PackageInternal {
		t.Errorf("a block directive should survive an empty spec, got %v", got.Scope)
	}
}

// TestMergeAccumulatesIgnores pins the half of Merge that differs from scope:
// ignores are unioned, not replaced, so a narrower directive on a spec cannot
// silently re-enable a rule the enclosing block turned off. The README and
// CLAUDE.md describe this behavior and must not drift from it.
func TestMergeAccumulatesIgnores(t *testing.T) {
	outer := directive.Decl{Ignores: []directive.Ignore{{}}}
	inner := directive.Decl{Ignores: []directive.Ignore{{Rules: []rule.Rule{rule.Unqualify}}}}

	got := outer.Merge(inner)
	if len(got.Ignores) != 2 {
		t.Fatalf("got %d ignores, want both the block's and the spec's", len(got.Ignores))
	}
	for _, r := range rule.All {
		if !got.Ignores[0].Covers(r) {
			t.Errorf("the block's bare ignore no longer covers %q after merging", r)
		}
	}
	if got.Ignores[1].Covers(rule.Boundary) {
		t.Error("the spec's ignore should still name only its own rules")
	}
	if len(outer.Ignores) != 1 || len(inner.Ignores) != 1 {
		t.Error("Merge must not mutate its inputs")
	}
}

func TestParseFileNamespace(t *testing.T) {
	f := directive.ParseFile(parse(t, "//declscope:namespace user\npackage repo\n\nfunc f() {}\n"))
	if !f.HasNamespace || f.Namespace != "user" {
		t.Fatalf("Namespace = %q, %v, want \"user\", true", f.Namespace, f.HasNamespace)
	}
	if len(f.Problems) != 0 {
		t.Errorf("unexpected problems: %v", f.Problems)
	}
}

// TestParseFileIgnore checks that a file-level ignore takes the same argument
// as the declaration-level one, so the directive means one thing wherever it
// is written.
func TestParseFileIgnore(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		covers  []rule.Rule
		misses  []rule.Rule
	}{
		{
			name:    "named rules",
			comment: "//declscope:ignore qualify,unqualify",
			covers:  []rule.Rule{rule.Qualify, rule.Unqualify},
			misses:  []rule.Rule{rule.Boundary},
		},
		{
			name:    "reach may be silenced too",
			comment: "//declscope:ignore boundary",
			covers:  []rule.Rule{rule.Boundary},
			misses:  []rule.Rule{rule.Qualify},
		},
		{
			name:    "bare covers everything",
			comment: "//declscope:ignore",
			covers:  rule.All,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := directive.ParseFile(parse(t, tt.comment+"\n\npackage repo\n\nfunc f() {}\n"))
			if len(f.Problems) != 0 {
				t.Fatalf("unexpected problems: %v", f.Problems)
			}
			if len(f.Ignores) != 1 {
				t.Fatalf("got %d ignores, want 1", len(f.Ignores))
			}
			for _, r := range tt.covers {
				if !f.Ignores[0].Covers(r) {
					t.Errorf("Covers(%q) = false, want true", r)
				}
			}
			for _, r := range tt.misses {
				if f.Ignores[0].Covers(r) {
					t.Errorf("Covers(%q) = true, want false", r)
				}
			}
		})
	}
}

// TestParseFileScope checks that a scope directive before the package clause is
// read as the file's default rather than reported. It is the honest form of
// what a shared utility file used to have to spell as an ignore: a scope states
// what the declarations are, where an ignore only stands a rule down.
func TestParseFileScope(t *testing.T) {
	f := directive.ParseFile(parse(t, "//declscope:package\n\npackage repo\n"))
	if len(f.Problems) != 0 {
		t.Fatalf("want no problem, got %v", f.Problems)
	}
	if !f.Scope.HasScope || f.Scope.Scope != scope.PackageInternal {
		t.Errorf("Scope = %+v, want package-internal", f.Scope)
	}
}

// TestParseFileCore checks the core directive, and that naming a namespace as
// well is refused: a core file's namespace is the core.
func TestParseFileCore(t *testing.T) {
	f := directive.ParseFile(parse(t, "//declscope:core\n\npackage repo\n"))
	if len(f.Problems) != 0 || !f.Core {
		t.Fatalf("Core = %v, problems = %v", f.Core, f.Problems)
	}
	f = directive.ParseFile(parse(t, "//declscope:core\n//declscope:namespace user\n\npackage repo\n"))
	if len(f.Problems) == 0 {
		t.Error("want a problem for core and namespace on one file")
	}
	f = directive.ParseFile(parse(t, "//declscope:core user\n\npackage repo\n"))
	if len(f.Problems) == 0 {
		t.Error("want a problem for an argument to core")
	}
}

func TestParseFileNamespaceProblems(t *testing.T) {
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
			f := directive.ParseFile(parse(t, tt.src))
			if len(f.Problems) == 0 {
				t.Error("want a problem, got none")
			}
		})
	}
}

// TestParseFileIgnoresDeclarations checks that a namespace directive after the
// package clause is not mistaken for a file-level one.
func TestParseFileIgnoresDeclarations(t *testing.T) {
	f := directive.ParseFile(parse(t, "package repo\n\n//declscope:namespace user\nfunc f() {}\n"))
	if f.HasNamespace {
		t.Error("a namespace directive below the package clause should not apply to the file")
	}
}
