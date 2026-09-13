//declscope:namespace analyzer

package internal

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
)

// ignoreSite is one physical ignore directive, however many declarations it
// reaches. A directive on a block is copied into every spec by Decl.Merge and
// one on `var a, b` is shared by both names, but it is still one comment, so it
// is judged once: unused only if it silenced nothing for any of them. Judged
// per target, it would be reported unused whenever any sibling did not need
// it, and a wholly unused one would be reported once per sibling.
type ignoreSite struct {
	ig directive.Ignore
	// decls names the declarations the directive reaches, in source order,
	// for the report. It is empty for a file-level directive, and for one
	// carried only by declarations declscope does not check (init, _, an
	// embedded field), which is then unused by construction.
	decls     []string
	fileLevel bool
	used      bool
}

// site returns the accounting entry for ig, keyed by where it is written.
func (c *collection) site(ig directive.Ignore) *ignoreSite {
	s, ok := c.ignores[ig.Pos]
	if !ok {
		s = &ignoreSite{ig: ig}
		c.ignores[ig.Pos] = s
	}
	return s
}

// parseDecl parses declaration-level directives from the given comment groups
// and does the bookkeeping that must happen once per physical comment: it
// registers every ignore, so that one attached to no checked declaration is
// still reported unused; records every problem, so that a block's bogus
// directive is reported once and not once per spec; and remembers the group as
// consumed, so that stray can tell which directives reached nothing.
//
// A group already consumed is skipped rather than parsed twice, so a comment
// that is reachable along two paths (a single-line spec's Comment is also the
// comment trailing its first line) yields one directive, not two.
func (c *collection) parseDecl(groups ...*ast.CommentGroup) directive.Decl {
	fresh := make([]*ast.CommentGroup, 0, len(groups))
	for _, g := range groups {
		if g == nil || c.consumed[g] {
			continue
		}
		c.consumed[g] = true
		fresh = append(fresh, g)
	}
	d := directive.ParseDecl(fresh...)
	for _, ig := range d.Ignores {
		c.site(ig)
	}
	c.problems = append(c.problems, d.Problems...)
	return d
}

// specGroups returns the comment groups a spec takes its directives from: its
// doc comment, the trailing comment go/parser attached to it, and any comment
// trailing its first or last line that the parser attached to nothing, such
// as one after the opening brace of a struct type. A comment the parser hung
// on something inside the spec — a field's doc or trailing comment — is that
// field's and is left for addFields, even when it shares the spec's first
// line.
func (c *collection) specGroups(pass *analysis.Pass, fi *fileInfo, spec ast.Spec) []*ast.CommentGroup {
	var own []*ast.CommentGroup
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.ValueSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	}
	return append(own, fi.looseTrailing(pass.Fset, spec, attached(spec)...)...)
}

// looseTrailing returns the comment groups on the first and last lines of
// node that go/parser attached to nothing — one after the opening brace of a
// struct type, after the parenthesis of a block, or after a function's
// closing brace. They belong to the declaration spanning those lines, the way
// a trailing comment on a one-line declaration does.
//
// attached lists the groups the parser did hang on something inside node,
// which win: a comment trailing a field on the same line as the brace is the
// field's, not the type's.
func (f *fileInfo) looseTrailing(fset *token.FileSet, node ast.Node, attached ...*ast.CommentGroup) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, pos := range []token.Pos{node.Pos(), node.End()} {
		g := f.trailingAt(fset, pos)
		if g == nil || slices.Contains(attached, g) || slices.Contains(out, g) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// attached returns the comment groups go/parser hung on a spec or on anything
// inside it, so that looseTrailing does not claim them for the enclosing
// declaration.
func attached(spec ast.Spec) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		out = append(out, spec.Doc, spec.Comment)
		var fields *ast.FieldList
		switch t := spec.Type.(type) {
		case *ast.StructType:
			fields = t.Fields
		case *ast.InterfaceType:
			fields = t.Methods
		}
		if fields != nil {
			for _, field := range fields.List {
				out = append(out, field.Doc, field.Comment)
			}
		}
	case *ast.ValueSpec:
		out = append(out, spec.Doc, spec.Comment)
	}
	return out
}

// stray reports every directive written after the package clause that no
// declaration consumed. Anything parseDecl saw is accounted for, whether or
// not it produced a target; what is left is a directive the author believes
// is in force and is not.
func (c *collection) stray() {
	for _, fi := range c.files {
		for _, g := range fi.file.Comments {
			// Comments before the package clause are the file's, and
			// directive.ParseFile has already judged them.
			if g.Pos() < fi.file.Package || c.consumed[g] {
				continue
			}
			c.problems = append(c.problems, directive.Stray(g)...)
		}
	}
}

// silenced reports whether any ignore directive covering t silences r.
//
// A member inherits the directives written on the type that owns it, so the
// chain runs declaration, then owning type, then file. Every level is
// consulted rather than stopping at the first hit, and every directive that
// covers the rule is marked used, so overlapping directives at different
// levels do not make each other look unused.
func (c *collection) silenced(t *target, r rule.Rule) bool {
	hit := c.ignored(t.dir.Ignores, r)
	// A field is written inside its type's declaration, so the type's ignores
	// contain it the way its scope directive does. A method is an ordinary
	// top-level declaration and its type reaches neither: the suppression
	// chain and the scope chain walk the same levels, so that a reader who
	// learns one has learned both.
	if t.kind == kindField {
		if owner, ok := c.byObj[t.ownerObj]; ok && owner != t {
			hit = c.ignored(owner.dir.Ignores, r) || hit
		}
	}
	return c.ignored(t.file.ignores, r) || hit
}

// ignored reports whether any directive silences r, marking every directive
// that does as used. All of them are marked, not just the first, so that
// overlapping directives are not reported as unused.
func (c *collection) ignored(ignores []directive.Ignore, r rule.Rule) bool {
	hit := false
	for _, ig := range ignores {
		if ig.Covers(r) {
			c.site(ig).used = true
			hit = true
		}
	}
	return hit
}

// reportUnusedIgnores reports every ignore directive that silenced nothing.
//
// Only a pass that sees every reference in the package can tell. The ordinary
// variant of a package with in-package _test.go files does not see the
// references those files make, so a directive needed only by a test would be
// reported unused there and reported necessary by the test variant, and the
// author could satisfy neither. That pass leaves the judgment to the test
// variant, on the same reasoning that keeps unmatched baseline entries
// unreported. Under -test (the default) the test variant runs and nothing is
// lost; with -test=false, a package with in-package tests gets no
// unused-ignore report at all, which is the only report that can be trusted.
func (c *collection) reportUnusedIgnores(pass *analysis.Pass) {
	if c.hasUnseenTests(pass) {
		return
	}
	sites := make([]*ignoreSite, 0, len(c.ignores))
	for _, s := range c.ignores {
		if !s.used {
			sites = append(sites, s)
		}
	}
	slices.SortFunc(sites, func(a, b *ignoreSite) int { return comparePos(pass.Fset, a.ig.Pos, b.ig.Pos) })
	for _, s := range sites {
		switch {
		case s.fileLevel:
			pass.Reportf(s.ig.Pos, "unused file-level %s", s.ig)
		case len(s.decls) == 0:
			pass.Reportf(s.ig.Pos, "unused %s: no checked declaration carries it", s.ig)
		default:
			pass.Reportf(s.ig.Pos, "unused %s on %s", s.ig, strings.Join(s.decls, ", "))
		}
	}
}
