// Package directive parses declscope comment directives.
//
// # Supported directives
//
// Declaration level, stating the scope explicitly instead of deriving it from
// the name:
//
//	//declscope:public
//	//declscope:package
//	//declscope:file
//
// Declaration level, suppressing diagnostics for the declaration. With no
// argument it silences every rule; with one it silences only the rules named,
// so that a declaration can opt out of one check while staying subject to the
// rest:
//
//	//declscope:ignore
//	//declscope:ignore demote
//	//declscope:ignore demote,prefix
//
// File level, placed before the package clause, overriding the namespace that
// would otherwise be derived from the file name:
//
//	//declscope:namespace user
//	package repo
//
// # Placement
//
// Declaration level directives go in the doc comment of a declaration or in a
// trailing comment on the same line:
//
//	//declscope:package
//	func helper() {}
//
//	func helper() {} //declscope:package
//
// A directive on a parenthesized var/const/type block applies to every spec in
// the block, and a directive on an individual spec overrides it.
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

// Merge layers a more specific Decl over a broader one, so that a directive on
// a spec wins over one on the enclosing block.
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
		ignore := Ignore{Pos: pos}
		for _, name := range strings.Split(arg, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			r, ok := rule.Parse(name)
			if !ok {
				d.problem(pos, fmt.Sprintf("unknown rule %q in declscope:ignore (want one of %s)",
					name, strings.Join(rule.Names(), ", ")))
				return
			}
			ignore.Rules = append(ignore.Rules, r)
		}
		d.Ignores = append(d.Ignores, ignore)

	case "namespace":
		d.problem(pos, "declscope:namespace must appear before the package clause")

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

// FileNamespace reports the namespace declared before the package clause.
// It returns ok=false when the file declares none.
func FileNamespace(file *ast.File) (name string, pos token.Pos, ok bool, problems []Problem) {
	for _, g := range file.Comments {
		// Directives after the package clause belong to declarations.
		if g.Pos() > file.Package {
			break
		}
		for _, c := range g.List {
			keyword, arg, found := split(c.Text)
			if !found || keyword != "namespace" {
				continue
			}
			switch {
			case arg == "":
				problems = append(problems, Problem{c.Pos(), "declscope:namespace requires a name"})
			case !isLowerIdent(arg):
				problems = append(problems, Problem{c.Pos(), fmt.Sprintf("namespace %q is not a valid lowerCamelCase identifier", arg)})
			case ok:
				problems = append(problems, Problem{c.Pos(), "duplicate declscope:namespace directive"})
			default:
				name, pos, ok = arg, c.Pos(), true
			}
		}
	}
	return name, pos, ok, problems
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
