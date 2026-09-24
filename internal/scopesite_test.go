package internal

import (
	"errors"
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// scopesiteRemovalPass is a pass over one file holding content, and the
// position of the directive written at the first "//declscope:" in it. The
// deletion is only a text edit, so nothing needs to parse.
func scopesiteRemovalPass(t *testing.T, content string) (*analysis.Pass, token.Pos) {
	t.Helper()
	off := strings.Index(content, "//declscope:")
	if off < 0 {
		t.Fatalf("no directive in %q", content)
	}
	fset := token.NewFileSet()
	tf := fset.AddFile("a.go", -1, len(content))
	tf.SetLinesForContent([]byte(content))
	pass := &analysis.Pass{
		Fset:     fset,
		ReadFile: func(string) ([]byte, error) { return []byte(content), nil },
	}
	return pass, tf.Pos(off)
}

// TestScopesiteRemoval pins the text the strict fix leaves. The analysistest
// goldens compare after gofmt, which would hide a doubled blank line or a
// changed line ending, so the bytes are compared here.
func TestScopesiteRemoval(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{
			name: "the bare // separating it from a doc comment goes with it",
			in:   "package x\n\n// f does.\n//\n//declscope:private\nfunc f() {}\n",
			want: "package x\n\n// f does.\nfunc f() {}\n",
		},
		{
			name: "the separator stays when a directive follows",
			in:   "package x\n\n// f does.\n//\n//declscope:private\n//declscope:ignore qualify\nfunc f() {}\n",
			want: "package x\n\n// f does.\n//\n//declscope:ignore qualify\nfunc f() {}\n",
		},
		{
			name: "the separator stays when the doc goes on",
			in:   "package x\n\n// f does.\n//\n//declscope:private\n// And more.\nfunc f() {}\n",
			want: "package x\n\n// f does.\n//\n// And more.\nfunc f() {}\n",
		},
		{
			name: "alone above a declaration, the blank line before it stays",
			in:   "package x\n\n//declscope:private\nfunc f() {}\n",
			want: "package x\n\nfunc f() {}\n",
		},
		{
			name: "at the start of the file, the blank line after it goes",
			in:   "//declscope:private\n\npackage x\n",
			want: "package x\n",
		},
		{
			name: "between blank lines, one of them goes",
			in:   "//go:build linux\n\n//declscope:private\n\npackage x\n",
			want: "//go:build linux\n\npackage x\n",
		},
		{
			name: "flush against the package clause, only its line goes",
			in:   "// Package x does.\n//\n//declscope:private\npackage x\n",
			want: "// Package x does.\npackage x\n",
		},
		{
			name: "trailing code, the comment and the space before it go",
			in:   "package x\n\ntype t struct {\n\tside int //declscope:private // why\n}\n",
			want: "package x\n\ntype t struct {\n\tside int\n}\n",
		},
		{
			name: "trailing code at the end of a file with no final newline",
			in:   "package x\n\nvar v int //declscope:private",
			want: "package x\n\nvar v int",
		},
		{
			name: "trailing code in a CRLF file, the line ending stays",
			in:   "package x\r\n\r\nvar v int //declscope:private\r\n",
			want: "package x\r\n\r\nvar v int\r\n",
		},
		{
			name: "alone in a CRLF file, with its separator",
			in:   "package x\r\n\r\n// f does.\r\n//\r\n//declscope:private\r\nfunc f() {}\r\n",
			want: "package x\r\n\r\n// f does.\r\nfunc f() {}\r\n",
		},
		{
			name: "at the start of a CRLF file, with the blank line after it",
			in:   "//declscope:private\r\n\r\npackage x\r\n",
			want: "package x\r\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pass, pos := scopesiteRemovalPass(t, tc.in)
			edit, ok := scopesiteRemoval(pass, pos)
			if !ok {
				t.Fatal("no edit")
			}
			tf := pass.Fset.File(pos)
			start, end := tf.Offset(edit.Pos), tf.Offset(edit.End)
			got := tc.in[:start] + string(edit.NewText) + tc.in[end:]
			if got != tc.want {
				t.Errorf("got\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestScopesiteRemovalFailsSafe pins that a file the pass cannot read as it
// was parsed yields no fix rather than a wrong one. A pass built by hand, as
// the survey builds one, has no ReadFile at all.
func TestScopesiteRemovalFailsSafe(t *testing.T) {
	const content = "package x\n\n//declscope:private\nfunc f() {}\n"
	for _, tc := range []struct {
		name  string
		setup func(*analysis.Pass, token.Pos) token.Pos
	}{
		{"no ReadFile", func(p *analysis.Pass, pos token.Pos) token.Pos {
			p.ReadFile = nil
			return pos
		}},
		{"a position in no file", func(_ *analysis.Pass, _ token.Pos) token.Pos {
			return token.NoPos
		}},
		{"a read error", func(p *analysis.Pass, pos token.Pos) token.Pos {
			p.ReadFile = func(string) ([]byte, error) { return nil, errors.New("gone") }
			return pos
		}},
		{"a file shorter than it was parsed", func(p *analysis.Pass, pos token.Pos) token.Pos {
			p.ReadFile = func(string) ([]byte, error) { return []byte("package x\n"), nil }
			return pos
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pass, pos := scopesiteRemovalPass(t, content)
			if edit, ok := scopesiteRemoval(pass, tc.setup(pass, pos)); ok {
				t.Errorf("got an edit %+v, want none", edit)
			}
		})
	}
}
