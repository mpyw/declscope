package internal

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/rule"
)

// collectBook is collect.go's half of the collection, embedded there.
//
//declscope:package // collection embeds it, and collection lives in the core
type collectBook struct {
	// consumed is every comment group some declaration or file took its
	// directives from; a directive outside them reached nothing.
	//
	//declscope:private // the type is widened only so the core can embed it
	consumed map[*ast.CommentGroup]bool
}

// collectFiles resolves each file's namespace. Generated files are excluded
// entirely: they are neither checked nor treated as reference sites, since a
// violation in generated code is not something the author can act on.
//
//declscope:package // the pipeline's first stage, driven from analyzer.go
func collectFiles(pass *analysis.Pass, opts Options) *collection {
	c := &collection{
		byFile: make(map[*ast.File]*fileInfo),
		byObj:  make(map[types.Object]*target),
		refs:   make(map[types.Object][]ref),
		idents: make(map[types.Object][]*ast.Ident),

		// Only this file's own half is built here. ignore.go and scopesite.go
		// fill theirs on first use, the way rename.go already did, so the
		// constructor never reaches into another stage's state.
		collectBook: collectBook{consumed: make(map[*ast.CommentGroup]bool)},
	}
	cores := make(map[string]bool)
	for _, f := range pass.Files {
		path := pass.Fset.Position(f.Pos()).Filename
		if ast.IsGenerated(f) || opts.Excluded(path) {
			continue
		}
		fi := &fileInfo{file: f, path: path, lineComments: make(map[int]*ast.CommentGroup)}
		fileDir := directive.ParseFile(f)
		fi.ignores = fileDir.Ignores
		for _, ig := range fileDir.Ignores {
			c.site(ig).fileLevel = true
		}
		c.problems = append(c.problems, fileDir.Problems...)
		for _, g := range f.Comments {
			line := pass.Fset.Position(g.Pos()).Line
			if _, seen := fi.lineComments[line]; !seen {
				fi.lineComments[line] = g
			}
		}
		fi.scope = fileDir.Scope
		if fileDir.Scope.HasScope {
			c.scopeSite(fileDir.Scope).fileLevel = true
		}
		fi.core = fileDir.Core
		switch {
		case fileDir.Core:
			// The core namespace has no name. Every core file shares it, which
			// is what makes having no prefix name exactly one unit.
			fi.ns = ""
		case fileDir.HasNamespace:
			fi.ns = fileDir.Namespace
		default:
			fi.ns = namespace.Of(path)
		}
		c.files = append(c.files, fi)
		c.byFile[f] = fi
		if fileDir.Core {
			cores[namespace.Of(path)] = true
		}
	}
	// A test file joins its subject's namespace, which is why a namespace is
	// derived from the stem rather than being the file name. //declscope:core
	// comes from a directive rather than from the stem, so the joining has to be
	// done here: without it client_test.go could not reach what client.go
	// declares, and the mechanical repair would be to widen the whole core.
	for _, fi := range c.files {
		if !fi.core && cores[namespace.Of(fi.path)] {
			fi.core, fi.ns = true, ""
		}
	}
	seen := make(map[string]bool)
	for _, fi := range c.files {
		if strings.HasSuffix(fi.path, "_test.go") {
			continue
		}
		seen[fi.key()] = true
	}
	c.namespaces = len(seen)
	return c
}

// collectTargets walks every declaration and resolves its scope.
//
//declscope:package // the pipeline's second stage, driven from analyzer.go
func (c *collection) collectTargets(pass *analysis.Pass, opts Options) {
	for _, fi := range c.files {
		for _, d := range fi.file.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				c.addFunc(pass, opts, fi, d)
			case *ast.GenDecl:
				c.addGenDecl(pass, opts, fi, d)
			}
		}
	}
	c.stray()
}

func (c *collection) addFunc(pass *analysis.Pass, opts Options, fi *fileInfo, d *ast.FuncDecl) {
	obj, ok := pass.TypesInfo.Defs[d.Name].(*types.Func)
	if !ok || d.Name.Name == "_" {
		return
	}
	dir := c.parseDecl(append([]*ast.CommentGroup{d.Doc}, fi.looseTrailing(pass.Fset, d)...)...)

	if d.Recv == nil {
		// init is not declared in package scope and can never be referenced.
		if d.Name.Name == "init" {
			return
		}
		sc, boundBy, boundAt := c.bind(opts, d.Name.Name, dir, directive.Decl{}, fi.scope)
		c.add(&target{
			obj: obj, ident: d.Name, kind: kindFunc, file: fi, dir: dir, anchor: d.Pos(),
			scope:      sc,
			boundBy:    boundBy,
			boundAt:    boundAt,
			renameable: true,
		})
		return
	}

	// A method is an ordinary top-level declaration that happens to name a
	// receiver: the file it is written in gives it its namespace, exactly as
	// for a func, and its type reaches neither its scope nor its ignores. The
	// receiver is still read, for the name a diagnostic prints.
	ownerObj := c.receiver(pass, obj)
	owner := ""
	if ownerObj != nil {
		owner = ownerObj.Name()
	}
	sc, boundBy, boundAt := c.bind(opts, d.Name.Name, dir, directive.Decl{}, fi.scope)
	c.add(&target{
		obj: obj, ident: d.Name, kind: kindMethod, file: fi,
		owner: owner, ownerObj: ownerObj, dir: dir, anchor: d.Pos(),
		scope:   sc,
		boundBy: boundBy,
		boundAt: boundAt,
	})
}

func (c *collection) addGenDecl(pass *analysis.Pass, opts Options, fi *fileInfo, d *ast.GenDecl) {
	if d.Tok == token.IMPORT {
		return
	}
	// A directive on the block applies to every spec. A spec's own scope
	// directive replaces the block's; its ignores are added to the block's.
	// A parenthesized block can carry a trailing directive on its own lines,
	// after "(" or ")"; an unparenthesized declaration's lines are its single
	// spec's, and are read below.
	grouped := d.Lparen.IsValid()
	outer := c.parseDecl(d.Doc)
	if grouped && len(d.Specs) > 0 {
		attached := append(c.attached(d.Specs[0]), c.attached(d.Specs[len(d.Specs)-1])...)
		outer = outer.Merge(c.parseDecl(fi.looseTrailing(pass.Fset, d, attached...)...))
	}

	for _, spec := range d.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			dir := outer.Merge(c.parseDecl(c.specGroups(pass, fi, spec)...))
			c.shadow(outer, dir)
			anchor := d.Pos()
			if grouped {
				anchor = spec.Pos()
			}
			if obj, ok := pass.TypesInfo.Defs[spec.Name]; ok && spec.Name.Name != "_" {
				sc, boundBy, boundAt := c.bind(opts, spec.Name.Name, dir, directive.Decl{}, fi.scope)
				c.add(&target{
					obj: obj, ident: spec.Name, kind: kindType, file: fi, dir: dir, anchor: anchor,
					scope:      sc,
					boundBy:    boundBy,
					boundAt:    boundAt,
					renameable: true,
				})
			}
			c.addMembers(pass, opts, fi, spec, pass.TypesInfo.Defs[spec.Name], dir)

		case *ast.ValueSpec:
			dir := outer.Merge(c.parseDecl(c.specGroups(pass, fi, spec)...))
			c.shadow(outer, dir)
			anchor := d.Pos()
			if grouped {
				anchor = spec.Pos()
			}
			k := kindVar
			if d.Tok == token.CONST {
				k = kindConst
			}
			for _, name := range spec.Names {
				obj, ok := pass.TypesInfo.Defs[name]
				if !ok || name.Name == "_" {
					continue
				}
				sc, boundBy, boundAt := c.bind(opts, name.Name, dir, directive.Decl{}, fi.scope)
				c.add(&target{
					obj: obj, ident: name, kind: k, file: fi, dir: dir, anchor: anchor,
					scope:      sc,
					boundBy:    boundBy,
					boundAt:    boundAt,
					renameable: true,
				})
			}
		}
	}
}

// addMembers registers the members a named type declares: a struct's fields,
// or an interface's method names.
//
// A member is written inside its type's declaration, so the file it is in is
// the type's: that is where it is, not a binding chosen for it. The type's
// directive therefore contains the member the way a var (...) block contains
// its specs, and is consulted before the file level.
//
// Two shapes qualify, and the ast spells them alike. A struct field is one. An
// interface's method name is the other: it is declared by the interface, only
// this package can spell it, and a file boundary around it is what the sealed
// interface idiom asks for. Satisfying the interface is not a use of the name
// — a method set is resolved, not written — so a type implementing it from
// another namespace crosses nothing. Naming the method does cross.
func (c *collection) addMembers(pass *analysis.Pass, opts Options, fi *fileInfo, spec *ast.TypeSpec, ownerObj types.Object, container directive.Decl) {
	var members []*ast.Field
	var k kind
	switch t := spec.Type.(type) {
	case *ast.StructType:
		if t.Fields == nil {
			return
		}
		members, k = t.Fields.List, kindField
	case *ast.InterfaceType:
		if t.Methods == nil {
			return
		}
		members, k = t.Methods.List, kindMethod
	default:
		return
	}
	for _, m := range members {
		// Parsed before the name check so that a directive on something
		// unnamed is accounted for: it reaches no checked declaration and is
		// reported unused, rather than dropped.
		dir := c.parseDecl(m.Doc, m.Comment)
		// An entry with no name is an embedded field, an embedded interface,
		// or an element of a type constraint. None declares a name of its own,
		// so there is nothing here to bound or to rename.
		if len(m.Names) == 0 {
			continue
		}
		for _, name := range m.Names {
			obj, ok := pass.TypesInfo.Defs[name]
			if !ok || name.Name == "_" {
				continue
			}
			sc, boundBy, boundAt := c.bind(opts, name.Name, dir, container, fi.scope)
			c.add(&target{
				obj: obj, ident: name, kind: k, file: fi,
				contained: true,
				owner:     spec.Name.Name, ownerObj: ownerObj, dir: dir,
				anchor:  m.Pos(),
				scope:   sc,
				boundBy: boundBy,
				boundAt: boundAt,
			})
		}
	}
}

// receiver resolves the type a method belongs to and the file declaring that
// type, along with that file's namespace. It falls back to the method's own
// file when the type cannot be traced to one in the package.
func (c *collection) receiver(_ *analysis.Pass, fn *types.Func) types.Object {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	t := sig.Recv().Type()
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	// A method on a generic type receives List[T], an instantiation of List
	// with its own type parameters. Obj() already names the origin's type name,
	// which is the object collectTargets registered.
	return named.Obj()
}

// add registers a target and names it on every ignore directive reaching it,
// so that an unused one can be reported with the declarations it was written
// for. Problems were recorded when the directives were parsed, once per
// comment rather than once per target sharing it.
func (c *collection) add(t *target) {
	c.targets = append(c.targets, t)
	c.byObj[t.obj] = t
	for _, ig := range t.dir.Ignores {
		s := c.site(ig)
		s.decls = append(s.decls, t.name())
	}
	if t.dir.HasScope {
		s := c.scopeSite(t.dir)
		s.decls = append(s.decls, t.name())
	}
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
		c.site(ig).siblings = d.Ignores
	}
	// A scope directive is registered here too, so that one written on something
	// declscope does not check — init, _, an embedded field — is reported unused
	// rather than dropped. Registering it only where a target is added would
	// lose exactly the cases the report exists for.
	if d.HasScope {
		c.scopeSite(d)
	}
	// An ignore and a problem parsed from the same comment are the author
	// answering their own directive: the ignore is consulted here, where the
	// declaration that carries both is still in hand. A problem is attached to
	// no declaration once it reaches the report, so the file level is the only
	// one that could answer for it there.
	if len(d.Problems) > 0 && c.ignored(d.Ignores, rule.Directive) {
		return d
	}
	c.problems = append(c.problems, d.Problems...)
	return d
}

// specGroups returns the comment groups a spec takes its directives from: its
// doc comment, the trailing comment go/parser attached to it, and any comment
// trailing its first or last line that the parser attached to nothing, such
// as one after the opening brace of a struct type. A comment the parser hung
// on something inside the spec — a field's doc or trailing comment — is that
// field's and is left for addMembers, even when it shares the spec's first
// line.
func (c *collection) specGroups(pass *analysis.Pass, fi *fileInfo, spec ast.Spec) []*ast.CommentGroup {
	var own []*ast.CommentGroup
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.ValueSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	}
	return append(own, fi.looseTrailing(pass.Fset, spec, c.attached(spec)...)...)
}

// trailingAt returns the comment group trailing the declaration starting at
// pos, if any.
func (f *fileInfo) trailingAt(fset *token.FileSet, pos token.Pos) *ast.CommentGroup {
	g, ok := f.lineComments[fset.Position(pos).Line]
	if !ok || g.Pos() < pos {
		return nil
	}
	return g
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
func (c *collection) attached(spec ast.Spec) []*ast.CommentGroup {
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

// collectRefs records every ident naming a tracked object, along with the file
// it appears in.
//
// An ident can play both roles at once: an embedded field's ident defines the
// field and uses the type, so Defs and Uses are consulted independently rather
// than one shadowing the other. Returning after a hit in Defs would drop the
// type use, and with it both a rename edit and a boundary diagnostic.
//
//declscope:package // the pipeline's third stage, driven from analyzer.go
func (c *collection) collectRefs(pass *analysis.Pass) {
	for _, fi := range c.files {
		ast.Inspect(fi.file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if obj := pass.TypesInfo.Defs[ident]; obj != nil {
				c.idents[obj] = append(c.idents[obj], ident)
			}
			obj := origin(pass.TypesInfo.Uses[ident])
			if obj == nil {
				return true
			}
			c.idents[obj] = append(c.idents[obj], ident)
			if tn := embeddedTypeName(obj); tn != nil {
				c.idents[tn] = append(c.idents[tn], ident)
			}
			if _, tracked := c.byObj[obj]; tracked {
				c.refs[obj] = append(c.refs[obj], ref{ident: ident, file: fi})
			}
			return true
		})
	}
}
