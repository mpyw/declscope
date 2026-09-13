// Package directive parses declscope comment directives.
//
// # Supported directives
//
// Declaration level, stating the scope explicitly instead of deriving it from
// the name:
//
//	//declscope:package
//	//declscope:private
//
// Declaration level, suppressing diagnostics for the declaration. With no
// argument it silences every rule; with one it silences only the rules named,
// so that a declaration can opt out of one check while staying subject to the
// rest:
//
//	//declscope:ignore
//	//declscope:ignore unqualify
//	//declscope:ignore unqualify,qualify
//
// File level, placed before the package clause, overriding the namespace that
// would otherwise be derived from the file name:
//
//	//declscope:namespace user
//	package repo
//
// File level, standing a whole file outside one or more rules. It takes the
// same argument as the declaration-level form, so a bare one silences
// everything in the file:
//
//	//declscope:ignore qualify,unqualify
//
//	package util
//
// # Placement
//
// Declaration level directives go in the doc comment of a declaration or in a
// trailing comment on its first or last line, which for a multi-line
// declaration is the line of its opening or closing brace:
//
//	//declscope:package
//	func helper() {}
//
//	func helper() {} //declscope:package
//
//	type user struct { //declscope:ignore boundary
//		name string
//	}
//
// A directive on a parenthesized var/const/type block applies to every spec in
// the block. A spec may carry directives of its own: a scope directive on the
// spec replaces the block's, while ignores accumulate, so the spec is covered
// by both its own and the block's.
//
// A directive written anywhere else after the package clause — separated from
// its declaration by a blank line, or inside a function body — binds to
// nothing and is reported as misplaced rather than dropped.
//
// A trailing "// reason" is allowed after any directive:
//
//	//declscope:package // shared with the reporting code
package directive

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

const prefix = "declscope:"

// Problem is a malformed or contradictory directive.
type Problem struct {
	Pos token.Pos
	Msg string
}

// Ignore is one ignore directive. An empty Rules silences every rule.
type Ignore struct {
	Pos   token.Pos
	Rules []rule.Rule
}

// Covers reports whether the directive silences r.
func (i Ignore) Covers(r rule.Rule) bool {
	return len(i.Rules) == 0 || slices.Contains(i.Rules, r)
}

// String renders the directive as written, for reporting it unused.
func (i Ignore) String() string {
	if len(i.Rules) == 0 {
		return "//declscope:ignore"
	}
	names := make([]string, 0, len(i.Rules))
	for _, r := range i.Rules {
		names = append(names, string(r))
	}
	return "//declscope:ignore " + strings.Join(names, ",")
}

// Decl holds the directives that apply to a single declaration.
type Decl struct {
	Scope    scope.Scope
	HasScope bool
	ScopePos token.Pos

	Ignores []Ignore

	Problems []Problem
}

// Merge layers a more specific Decl over a broader one, as for a spec inside a
// parenthesized block. A scope directive on the inner Decl replaces the outer
// one's, since a declaration has exactly one scope. Ignores are unioned: a
// narrower ignore must not silently re-enable a rule the broader one turned
// off, so the inner Decl's are added to the outer's rather than replacing them.
func (d Decl) Merge(inner Decl) Decl {
	out := d
	if inner.HasScope {
		out.Scope, out.HasScope, out.ScopePos = inner.Scope, true, inner.ScopePos
	}
	out.Ignores = append(append([]Ignore(nil), d.Ignores...), inner.Ignores...)
	out.Problems = append(append([]Problem(nil), d.Problems...), inner.Problems...)
	return out
}

// ParseDecl collects declaration level directives from the given comment
// groups. Nil groups are skipped, so doc and trailing comments can be passed
// together.
func ParseDecl(groups ...*ast.CommentGroup) Decl {
	var d Decl
	for _, g := range groups {
		if g == nil {
			continue
		}
		for _, c := range g.List {
			keyword, arg, ok := split(c.Text)
			if !ok {
				continue
			}
			d.consume(c.Pos(), keyword, arg)
		}
	}
	return d
}

func (d *Decl) consume(pos token.Pos, keyword, arg string) {
	switch keyword {
	case "ignore":
		ignore, problem := parseIgnore(pos, arg)
		if problem != nil {
			d.Problems = append(d.Problems, *problem)
			return
		}
		d.Ignores = append(d.Ignores, ignore)

	case "namespace":
		d.problem(pos, "declscope:namespace must appear before the package clause")

	case "core":
		d.problem(pos, "declscope:core must appear before the package clause")

	default:
		s, ok := scope.Parse(keyword)
		if !ok {
			d.problem(pos, fmt.Sprintf("unknown directive declscope:%s", keyword))
			return
		}
		if arg != "" {
			d.problem(pos, fmt.Sprintf("%s takes no argument", s.Directive()))
			return
		}
		if d.HasScope && d.Scope != s {
			d.problem(pos, fmt.Sprintf("conflicting scope directives: %s and %s", d.Scope.Directive(), s.Directive()))
			return
		}
		d.Scope, d.HasScope, d.ScopePos = s, true, pos
	}
}

func (d *Decl) problem(pos token.Pos, msg string) {
	d.Problems = append(d.Problems, Problem{Pos: pos, Msg: msg})
}

// Stray reports every directive in a comment group that reached no
// declaration and no file: one separated from its declaration by a blank line,
// one inside a function body, one on an import. Such a directive would
// otherwise be dropped without a word, and for a suppression that is the worst
// outcome, since the author believes something is silenced.
func Stray(g *ast.CommentGroup) []Problem {
	var out []Problem
	for _, c := range g.List {
		keyword, _, ok := split(c.Text)
		if !ok {
			continue
		}
		msg := fmt.Sprintf("misplaced declscope:%s: no declaration here for it to bind to; "+
			"write it in a declaration's doc comment or trailing its first or last line", keyword)
		if keyword == "namespace" {
			msg = "declscope:namespace must appear before the package clause"
		}
		out = append(out, Problem{Pos: c.Pos(), Msg: msg})
	}
	return out
}

// File holds the directives that apply to a whole file.
type File struct {
	Namespace    string
	HasNamespace bool
	NamespacePos token.Pos

	// Core marks the file as part of the package's core namespace, whose prefix
	// is empty. Several files may carry it and they share the one namespace,
	// the way //declscope:namespace merges files under a name; the core has
	// none, which is what puts it outside the naming rules.
	Core    bool
	CorePos token.Pos

	// Scope is the file-level scope directive. It is a default for what the
	// file declares, not a blanket: a declaration may still state its own, and
	// a field takes its type's first.
	Scope Decl

	Ignores []Ignore

	Problems []Problem
}

// ParseFile collects the directives written before the package clause.
func ParseFile(file *ast.File) File {
	var f File
	for _, g := range file.Comments {
		// Directives after the package clause belong to declarations.
		if g.Pos() > file.Package {
			break
		}
		for _, c := range g.List {
			keyword, arg, found := split(c.Text)
			if !found {
				continue
			}
			switch keyword {
			case "namespace":
				f.namespace(c.Pos(), arg)
			case "core":
				f.core(c.Pos(), arg)
			case "ignore":
				f.ignore(c.Pos(), arg)
			case "package", "private":
				f.scope(c.Pos(), keyword, arg)
			default:
				f.problem(c.Pos(), fmt.Sprintf("declscope:%s is not a file-level directive", keyword))
			}
		}
	}
	// A core file's namespace is the core, so naming one as well contradicts it
	// rather than adding to it.
	if f.Core && f.HasNamespace {
		f.problem(f.CorePos, "conflicting namespace directives: a core file's namespace is the core")
	}
	return f
}

func (f *File) namespace(pos token.Pos, arg string) {
	switch {
	case arg == "":
		f.problem(pos, "declscope:namespace requires a name")
	case !isLowerIdent(arg):
		f.problem(pos, fmt.Sprintf("namespace %q is not a valid lowerCamelCase identifier", arg))
	case f.HasNamespace:
		f.problem(pos, "duplicate declscope:namespace directive")
	default:
		f.Namespace, f.NamespacePos, f.HasNamespace = arg, pos, true
	}
}

// core joins the file to the package's core namespace. A core file's namespace
// is the core, so naming one as well is a contradiction rather than an
// addition, and the two directives conflict.
func (f *File) core(pos token.Pos, arg string) {
	switch {
	case arg != "":
		f.problem(pos, "//declscope:core takes no argument")
	case f.Core:
		f.problem(pos, "duplicate declscope:core directive")
	default:
		f.Core, f.CorePos = true, pos
	}
}

// scope reads a file-level scope directive, the default for what the file
// declares.
func (f *File) scope(pos token.Pos, keyword, arg string) {
	sc, ok := scope.Parse(keyword)
	switch {
	case !ok:
		f.problem(pos, fmt.Sprintf("declscope:%s is not a scope", keyword))
	case arg != "":
		f.problem(pos, fmt.Sprintf("//declscope:%s takes no argument", keyword))
	case f.Scope.HasScope && f.Scope.Scope != sc:
		f.problem(pos, fmt.Sprintf("conflicting scope directives: %s and //declscope:%s on one file",
			f.Scope.Scope.Directive(), keyword))
	default:
		f.Scope.Scope, f.Scope.HasScope, f.Scope.ScopePos = sc, true, pos
	}
}

func (f *File) ignore(pos token.Pos, arg string) {
	ignore, problem := parseIgnore(pos, arg)
	if problem != nil {
		f.Problems = append(f.Problems, *problem)
		return
	}
	f.Ignores = append(f.Ignores, ignore)
}

// parseIgnore reads an ignore directive's rule list. Both levels share it, so
// //declscope:ignore means the same thing wherever it is written: named rules
// only, or everything when it names none.
func parseIgnore(pos token.Pos, arg string) (Ignore, *Problem) {
	ignore := Ignore{Pos: pos}
	for _, name := range strings.Split(arg, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		r, ok := rule.Parse(name)
		if !ok {
			return Ignore{}, &Problem{Pos: pos, Msg: fmt.Sprintf("unknown rule %q in declscope:ignore (want one of %s)",
				name, strings.Join(rule.Names(), ", "))}
		}
		ignore.Rules = append(ignore.Rules, r)
	}
	return ignore, nil
}

func (f *File) problem(pos token.Pos, msg string) {
	f.Problems = append(f.Problems, Problem{Pos: pos, Msg: msg})
}

// split extracts the keyword and argument from a comment holding a declscope
// directive. Both line and block comments are accepted; without the block form
// a /*declscope:package*/ comment would be a silent no-op rather than an error.
func split(text string) (keyword, arg string, ok bool) {
	if after, cut := strings.CutPrefix(text, "/*"); cut {
		text = strings.TrimSuffix(after, "*/")
	} else {
		text = strings.TrimPrefix(text, "//")
	}
	body, ok := strings.CutPrefix(strings.TrimSpace(text), prefix)
	if !ok {
		return "", "", false
	}
	// Drop an explanatory trailing comment: //declscope:package // reason
	if i := strings.Index(body, "//"); i >= 0 {
		body = body[:i]
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return "", "", false
	}
	return fields[0], strings.Join(fields[1:], " "), true
}

func isLowerIdent(s string) bool {
	if !token.IsIdentifier(s) {
		return false
	}
	return !ast.IsExported(s)
}
