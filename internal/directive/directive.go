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
//	//declscope:ignore qualify
//	//declscope:ignore boundary,qualify
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
//	//declscope:ignore boundary,qualify
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
// Only Go's directive form, //declscope:name with a lowercase name and no
// spaces, on a line comment, is read. Any other comment whose text opens
// with the tool's prefix is reported as malformed.
//
// A trailing "// reason" is allowed after any directive:
//
//	//declscope:package // shared with the reporting code
//
// # Linkname
//
// [Linknamed] reads the other directives that bear on a declaration's name:
// //go:linkname and cgo's //export, which name it as text.
package directive

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// tool is the tool part of a declscope directive, //tool:name args.
const tool = "declscope"

// Problem is a malformed or contradictory directive, or one that decides
// nothing.
type Problem struct {
	Pos token.Pos
	Msg string

	// Rule is the rule the report carries: directive for what this package
	// finds, unused for a directive the analysis finds deciding nothing.
	Rule rule.Rule

	// Fixes is at most one suggested fix. Only a redundant scope directive
	// under rules.unused: strict carries one, which deletes it.
	Fixes []analysis.SuggestedFix
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

	// Beneath is the scope directive Merge replaced: a block's, when the spec
	// states its own. Resolution never reads it, since the spec's wins. It is
	// kept for the one question that does: what the spec would take if its
	// own directive were deleted.
	Beneath *Decl
}

// BeneathScope returns the scope directive Merge replaced, or the zero Decl.
func (d Decl) BeneathScope() Decl {
	if d.Beneath == nil {
		return Decl{}
	}
	return *d.Beneath
}

// Merge layers a more specific Decl over a broader one, as for a spec inside a
// parenthesized block. A scope directive on the inner Decl replaces the outer
// one's, since a declaration has exactly one scope. Ignores are unioned: a
// narrower ignore must not silently re-enable a rule the broader one turned
// off, so the inner Decl's are added to the outer's rather than replacing them.
func (d Decl) Merge(inner Decl) Decl {
	out := d
	if inner.HasScope {
		if d.HasScope {
			out.Beneath = &Decl{Scope: d.Scope, HasScope: true, ScopePos: d.ScopePos}
		}
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
			keyword, arg, malformed, ok := split(c.Text)
			switch {
			case !ok:
			case malformed:
				d.problem(c.Pos(), malformedMessage)
			default:
				d.consume(c.Pos(), keyword, arg)
			}
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
	d.Problems = append(d.Problems, Problem{Pos: pos, Msg: msg, Rule: rule.Directive})
}

// Stray reports every directive in a comment group that reached no
// declaration and no file: one separated from its declaration by a blank line,
// one inside a function body, one on an import. Such a directive would
// otherwise be dropped without a word, and for a suppression that is the worst
// outcome, since the author believes something is silenced.
func Stray(g *ast.CommentGroup) []Problem {
	var out []Problem
	for _, c := range g.List {
		keyword, _, malformed, ok := split(c.Text)
		if !ok {
			continue
		}
		if malformed {
			out = append(out, Problem{Pos: c.Pos(), Msg: malformedMessage, Rule: rule.Directive})
			continue
		}
		msg := fmt.Sprintf("misplaced declscope:%s: no declaration here for it to bind to; "+
			"write it in a declaration's doc comment or trailing its first or last line", keyword)
		if keyword == "namespace" {
			msg = "declscope:namespace must appear before the package clause"
		}
		out = append(out, Problem{Pos: c.Pos(), Msg: msg, Rule: rule.Directive})
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
			keyword, arg, malformed, found := split(c.Text)
			if !found {
				continue
			}
			if malformed {
				f.problem(c.Pos(), malformedMessage)
				continue
			}
			// scope.Parse is asked first, so that the keywords naming a
			// scope are listed in one place rather than restated here.
			if sc, ok := scope.Parse(keyword); ok {
				f.scope(c.Pos(), sc, keyword, arg)
				continue
			}
			switch keyword {
			case "namespace":
				f.namespace(c.Pos(), arg)
			case "core":
				f.core(c.Pos(), arg)
			case "ignore":
				f.ignore(c.Pos(), arg)
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
// declares. The caller has already resolved the keyword, which is why there is
// no branch here for one that names no scope.
func (f *File) scope(pos token.Pos, sc scope.Scope, keyword, arg string) {
	switch {
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
			return Ignore{}, &Problem{Pos: pos, Rule: rule.Directive, Msg: fmt.Sprintf("unknown rule %q in declscope:ignore (want one of %s)",
				name, strings.Join(rule.Names(), ", "))}
		}
		ignore.Rules = append(ignore.Rules, r)
	}
	return ignore, nil
}

func (f *File) problem(pos token.Pos, msg string) {
	f.Problems = append(f.Problems, Problem{Pos: pos, Msg: msg, Rule: rule.Directive})
}

// split extracts the keyword and argument from a comment holding a declscope
// directive.
//
// A comment is addressed to declscope when its body, after the // or /*,
// opens with declscope: once any space is skipped. It is a directive when
// ast.ParseDirective reads it as it stands, once an explanatory trailing
// comment is dropped: //declscope:package // reason. Any other addressed
// comment is malformed, and is reported rather than silently doing nothing.
func split(text string) (keyword, arg string, malformed, ok bool) {
	body, line := strings.CutPrefix(text, "//")
	if !line {
		body = strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
	}
	if i := strings.Index(body, "//"); i >= 0 {
		body = body[:i]
	}
	if !strings.HasPrefix(strings.TrimSpace(body), tool+":") {
		return "", "", false, false
	}
	if d, parsed := ast.ParseDirective(token.NoPos, "//"+body); line && parsed && d.Tool == tool {
		return d.Name, strings.Join(strings.Fields(d.Args), " "), false, true
	}
	return "", "", true, true
}

// malformedMessage is the report on a comment addressed to declscope that is
// no directive.
const malformedMessage = "malformed declscope directive: write it as //declscope:name"

// Linknamed returns every local name that a //go:linkname or cgo //export
// directive in files binds. Either directive names a declaration as text, from
// code the analysis does not read, so a rename cannot follow it and a use
// through it is never spelled.
func Linknamed(files []*ast.File) map[string]bool {
	names := make(map[string]bool)
	for _, f := range files {
		for _, g := range f.Comments {
			for _, c := range g.List {
				if name, ok := linknameLocal(c.Text); ok {
					names[name] = true
				}
			}
		}
	}
	return names
}

// linknameLocal returns the local name a //go:linkname local target or an
// //export Name comment binds. //export predates the tool:name form, so
// ast.ParseDirective does not read it and it is matched by prefix.
func linknameLocal(text string) (string, bool) {
	args, ok := strings.CutPrefix(text, "//export ")
	if !ok {
		d, isDirective := ast.ParseDirective(token.NoPos, text)
		if !isDirective || d.Tool != "go" || d.Name != "linkname" {
			return "", false
		}
		args = d.Args
	}
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}

// isLowerIdent reports whether a namespace may be spelled this way.
//
// A keyword passes. A namespace is a file stem read as lowerCamelCase, not a
// Go identifier, and import.go, map.go and range.go are all ordinary file
// names; the namespaces they derive could not otherwise be spelled in a
// directive at all. token.IsIdentifier refuses keywords, so it is asked only
// about the shape of the word.
func isLowerIdent(s string) bool {
	if !token.IsIdentifier(s) && !token.IsKeyword(s) {
		return false
	}
	return !ast.IsExported(s)
}
