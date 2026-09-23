package directive_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
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
		{"with reason", "//declscope:package // shared with the reporter", scope.PackageInternal, true},
		{"reason flush against it", "//declscope:package// shared", scope.PackageInternal, true},
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
		{"core on a declaration", "//declscope:core"},
		{"conflicting scopes", "//declscope:package\n//declscope:private"},
		{"keyword with a suffix", "//declscope:packagex"},
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

// TestParseDeclNotADirective checks that a comment addressed to another tool,
// or to none, is neither read as a directive nor reported.
func TestParseDeclNotADirective(t *testing.T) {
	for _, comment := range []string{
		"//declscopex:package",
		"//xdeclscope:package",
		"// see declscope:package for why",
		"//go:generate declscope:package",
	} {
		t.Run(comment, func(t *testing.T) {
			fn := firstFunc(t, "package p\n\n"+comment+"\nfunc f() {}\n")
			d := directive.ParseDecl(fn.Doc)
			if d.HasScope || len(d.Ignores) != 0 || len(d.Problems) != 0 {
				t.Errorf("got %+v, want nothing", d)
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
			comment: "//declscope:ignore qualify",
			covers:  []rule.Rule{rule.Qualify},
			misses:  []rule.Rule{rule.Boundary, rule.Directive},
		},
		{
			name:    "several rules",
			comment: "//declscope:ignore boundary,qualify",
			covers:  []rule.Rule{rule.Boundary, rule.Qualify},
			misses:  []rule.Rule{rule.Directive},
		},
		{
			name:    "spaces around the separator",
			comment: "//declscope:ignore boundary, qualify",
			covers:  []rule.Rule{rule.Boundary, rule.Qualify},
			misses:  []rule.Rule{rule.Directive},
		},
		{
			name:    "with a reason",
			comment: "//declscope:ignore qualify // the namespace word is part of the concept",
			covers:  []rule.Rule{rule.Qualify},
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
	fn := firstFunc(t, "package p\n\n//declscope:ignore boundary\n//declscope:ignore qualify\nfunc f() {}\n")
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
	inner := directive.Decl{Ignores: []directive.Ignore{{Rules: []rule.Rule{rule.Qualify}}}}

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
			comment: "//declscope:ignore qualify,boundary",
			covers:  []rule.Rule{rule.Qualify, rule.Boundary},
			misses:  []rule.Rule{rule.Directive},
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
// read as the file's default rather than reported. It is what a wholly shared
// utility file wants instead of an ignore: a scope states what the declarations
// are, where an ignore only stands a rule down.
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
	f = directive.ParseFile(parse(t, "//declscope:core\n//declscope:core\n\npackage repo\n"))
	if len(f.Problems) == 0 {
		t.Error("want a problem for a repeated core directive")
	}
}

// TestParseFileProblems checks what a file-level directive is answered with
// when it cannot be honoured. Each message names the directive as written, so
// that the reader is told which line to change and not merely that one is
// wrong.
func TestParseFileProblems(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "argument where none is taken",
			src:  "//declscope:package everything\n\npackage repo\n",
			want: "//declscope:package takes no argument",
		},
		{
			name: "two scopes on one file",
			src:  "//declscope:package\n//declscope:private\n\npackage repo\n",
			want: "conflicting scope directives: //declscope:package and //declscope:private on one file",
		},
		{
			// A declaration-level keyword written before the package clause
			// reaches nothing: the file levels are namespace, core, ignore and
			// a scope, and ignore is the only one a declaration shares.
			name: "declaration-level keyword",
			src:  "//declscope:bogus\n\npackage repo\n",
			want: "declscope:bogus is not a file-level directive",
		},
		{
			name: "keyword with a suffix",
			src:  "//declscope:packagex\n\npackage repo\n",
			want: "declscope:packagex is not a file-level directive",
		},
		{
			name: "ignore of an unknown rule",
			src:  "//declscope:ignore nosuchrule\n\npackage repo\n",
			want: "nosuchrule",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := directive.ParseFile(parse(t, tt.src))
			if len(f.Problems) != 1 {
				t.Fatalf("got %d problems, want 1: %v", len(f.Problems), f.Problems)
			}
			if !strings.Contains(f.Problems[0].Msg, tt.want) {
				t.Errorf("message is %q, want it to name %q", f.Problems[0].Msg, tt.want)
			}
		})
	}
}

// TestParseFileScopeSurvivesRepetition checks that the same scope written
// twice is not a conflict. Only a second scope that disagrees is.
func TestParseFileScopeSurvivesRepetition(t *testing.T) {
	f := directive.ParseFile(parse(t, "//declscope:package\n//declscope:package\n\npackage repo\n"))
	if len(f.Problems) != 0 {
		t.Errorf("want no problem, got %v", f.Problems)
	}
	if !f.Scope.HasScope || f.Scope.Scope != scope.PackageInternal {
		t.Errorf("Scope = %+v, want package-internal", f.Scope)
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

// TestParseFileNamespaceKeyword checks that a Go keyword is a namespace a
// directive may spell. A namespace is a file stem read as lowerCamelCase, and
// import.go is an ordinary file name; refusing the word would leave the
// namespace it derives unspellable, which a file joining that namespace needs.
func TestParseFileNamespaceKeyword(t *testing.T) {
	for _, word := range []string{"import", "map", "range", "type"} {
		f := directive.ParseFile(parse(t, "//declscope:namespace "+word+"\npackage repo\n"))
		if len(f.Problems) != 0 {
			t.Errorf("%s: %v", word, f.Problems)
		}
		if f.Namespace != word {
			t.Errorf("namespace = %q, want %q", f.Namespace, word)
		}
	}
}

// TestParseDeclSkipsNilGroups checks that a caller may pass a node's doc and
// trailing comments together without checking either for nil. Most
// declarations have one and not the other.
func TestParseDeclSkipsNilGroups(t *testing.T) {
	fn := firstFunc(t, "package p\n\n//declscope:private\nfunc f() {}\n")
	d := directive.ParseDecl(nil, fn.Doc, nil)
	if !d.HasScope || d.Scope != scope.Private {
		t.Errorf("Scope = %v, %v, want private, true", d.Scope, d.HasScope)
	}
}

// TestMalformed checks that a comment addressed to declscope in any form but
// Go's canonical //tool:name is reported at every level, with the canonical
// spelling, and has no effect.
func TestMalformed(t *testing.T) {
	tests := []struct {
		comment string
		want    string
	}{
		{"// declscope:package", "write //declscope:package"},
		{"//\tdeclscope:package", "write //declscope:package"},
		{"//declscope: package", "write //declscope:package"},
		{"// declscope: private // why", "write //declscope:private"},
		{"/*declscope:package*/", "write //declscope:package"},
		{"/* declscope:ignore boundary, qualify */", "write //declscope:ignore boundary, qualify"},
		{"/*declscope:namespace user // why*/", "write //declscope:namespace user"},
		// Directive names are lowercase. The name is never guessed, so the
		// comment is named as written, without its reason.
		{"//declscope:Package", "//declscope:Package"},
		{"//declscope:Package // why", "//declscope:Package"},
		{"// declscope:Package", "// declscope:Package"},
		{"/*declscope:Package*/", "/*declscope:Package*/"},
		// A comment addressed to declscope with no name is reported too.
		{"//declscope:", "//declscope:"},
		{"//declscope: // reason", "//declscope:"},
		{"// declscope:", "// declscope:"},
		{"/*declscope:*/", "/*declscope:*/"},
	}
	for _, tt := range tests {
		want := "malformed directive: " + tt.want
		t.Run(tt.comment, func(t *testing.T) {
			check := func(level string, problems []directive.Problem) {
				t.Helper()
				if len(problems) != 1 || problems[0].Msg != want {
					t.Errorf("%s: got %v, want one %q", level, problems, want)
				}
			}

			d := directive.ParseDecl(firstFunc(t, "package p\n\n"+tt.comment+"\nfunc f() {}\n").Doc)
			check("declaration", d.Problems)
			if d.HasScope || len(d.Ignores) != 0 {
				t.Errorf("declaration: took effect: %+v", d)
			}

			f := directive.ParseFile(parse(t, tt.comment+"\n\npackage repo\n"))
			check("file", f.Problems)
			if f.HasNamespace || f.Scope.HasScope || len(f.Ignores) != 0 {
				t.Errorf("file: took effect: %+v", f)
			}

			check("stray", directive.Stray(&ast.CommentGroup{List: []*ast.Comment{{Text: tt.comment}}}))
		})
	}
}

// TestParseFileWithReason checks that a file-level directive may carry a
// trailing reason, as a declaration-level one may.
func TestParseFileWithReason(t *testing.T) {
	f := directive.ParseFile(parse(t, "//declscope:namespace user // pinned against a rename\n\npackage repo\n"))
	if len(f.Problems) != 0 || f.Namespace != "user" {
		t.Errorf("namespace = %q, problems = %v, want user and none", f.Namespace, f.Problems)
	}
}

// TestStray checks that a misplaced directive is reported, with or without a
// reason, and that a lookalike for another tool is left alone.
func TestStray(t *testing.T) {
	tests := []struct {
		comment string
		want    int
	}{
		{"//declscope:ignore", 1},
		{"//declscope:ignore // reason", 1},
		{"//declscopex:ignore", 0},
		{"// see declscope:ignore", 0},
		{"// an ordinary comment", 0},
	}
	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			g := &ast.CommentGroup{List: []*ast.Comment{{Text: tt.comment}}}
			got := directive.Stray(g)
			if len(got) != tt.want {
				t.Errorf("got %d problems, want %d: %v", len(got), tt.want, got)
			}
			for _, p := range got {
				if !strings.HasPrefix(p.Msg, "misplaced declscope:ignore") {
					t.Errorf("message is %q, want a misplaced report", p.Msg)
				}
			}
		})
	}
}

// TestLinknamed checks which local names a //go:linkname or //export binds,
// and that neither is matched by its spelling alone.
func TestLinknamed(t *testing.T) {
	src := `package p

import _ "unsafe"

//go:linkname clock runtime.nanotime
func clock() int64

//go:linkname pulled
func pulled() int { return 0 }

//export Tick
func Tick() {}

//go:linkname
//go:linknamex decoy
//go:noinline
// go:linkname spaced
//export
//exportx Decoy
func other() {}
`
	got := directive.Linknamed([]*ast.File{parse(t, src)})
	want := map[string]bool{"clock": true, "pulled": true, "Tick": true}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("%s is missing from %v", name, got)
		}
	}
}
