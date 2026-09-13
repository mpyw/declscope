//declscope:namespace analyzer

package internal

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// fileInfo is a source file together with the namespace it belongs to.
type fileInfo struct {
	file *ast.File
	path string
	ns   string

	// lineComments indexes every comment group by the line it starts on, so
	// that trailing directives can be found on declarations that carry no
	// comment field of their own, such as ast.FuncDecl.
	lineComments map[int]*ast.CommentGroup

	// ignores stands the whole file outside a naming rule. They apply on top
	// of whatever each declaration says for itself. Whether each silenced
	// anything is accounted for in collection.ignores.
	ignores []directive.Ignore

	// scope is the file-level scope directive, if any. It is a default for what
	// the file declares, not a blanket: any declaration may still state its own,
	// and a field takes its type's before the file is consulted.
	scope directive.Decl

	// core marks the file as part of the package's core namespace, whose label
	// is empty. Several files may carry it and they share the one namespace.
	core bool
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

// key identifies the namespace for comparison.
//
// The core namespace has no name, and every core file shares it, so they share
// one key. A file whose name has no stem at all also has no namespace, but must
// not be treated as sharing one with every other such file, so it falls back to
// its own path.
func (f *fileInfo) key() string {
	if f.core {
		return "\x00core"
	}
	if f.ns != "" {
		return f.ns
	}
	return "\x00" + f.path
}

// kind describes what a target declares, for diagnostic wording.
type kind string

const (
	kindFunc   kind = "func"
	kindType   kind = "type"
	kindVar    kind = "var"
	kindConst  kind = "const"
	kindMethod kind = "method"
	kindField  kind = "field"
)

// target is a declaration whose scope declscope enforces.
type target struct {
	obj   types.Object
	ident *ast.Ident
	kind  kind

	// file is where the declaration is written.
	file *fileInfo
	// ownerNS is the namespace that bounds the declaration: the namespace of
	// the file it is written in. A field is written inside its type's
	// declaration, so for a field that is the type's file; a method is an
	// ordinary top-level declaration and takes its own file's, like a func.
	ownerNS  string
	ownerKey string
	// ownerFile is the file that namespace comes from, which is the file the
	// declaration is written in for every kind.
	ownerFile *fileInfo
	// owner names the type a member belongs to, empty for package-level.
	owner string
	// ownerObj is that type's object, through which a member inherits the
	// ignore directives written on its type.
	ownerObj types.Object

	scope scope.Scope
	// boundBy is the directive that supplied the scope, zero when the configured
	// default did. It is not always t.dir: a field takes its type's and any
	// declaration takes its file's, and a diagnostic that named the declaration's
	// own directive in those cases would point at a comment that is not there.
	boundBy directive.Decl
	// subject says whether the declaration is declscope's business at all. A
	// declaration reachable from outside the package is not: its reach is
	// already published, and no boundary the analysis can check lies inside it.
	subject bool
	// dir holds the directives reaching the declaration. Its Ignores may be
	// shared with sibling targets — a block's directive reaches every spec —
	// so whether one silenced anything is tracked per physical directive in
	// collection.ignores, never per target.
	dir directive.Decl

	// anchor is where a scope directive would be inserted.
	anchor token.Pos
	// renameable is false for members, whose fix is never a rename.
	renameable bool
}

func (t *target) name() string {
	if t.owner != "" {
		return t.owner + "." + t.obj.Name()
	}
	return t.obj.Name()
}

// ref is a use of a target from somewhere in the package.
type ref struct {
	ident *ast.Ident
	file  *fileInfo
}

type collection struct {
	files   []*fileInfo
	byFile  map[*ast.File]*fileInfo
	targets []*target
	byObj   map[types.Object]*target
	refs    map[types.Object][]ref
	// idents holds every ident naming an object, definition included, so a
	// rename can rewrite all of them.
	idents   map[types.Object][]*ast.Ident
	problems []directive.Problem

	// ignores is every ignore directive in the package, keyed by where it is
	// written, so that one shared by several declarations is judged once.
	ignores map[token.Pos]*ignoreSite
	// scopes is the same accounting for scope directives: one entry per
	// physical comment, marked when something in its reach takes its scope.
	scopes map[token.Pos]*scopeSite
	// consumed is every comment group some declaration or file took its
	// directives from; a directive outside them reached nothing.
	consumed map[*ast.CommentGroup]bool

	// unseenTests reports whether the package directory holds in-package
	// _test.go files that are not in pass.Files, which is what the non-test
	// variant of a package with tests sees. Both the rename fix and the
	// unused-ignore report defer to the test variant when it does.
	unseenTests     bool
	unseenTestsDone bool

	// namespaces is how many distinct namespaces the package's non-test files
	// declare, which is how many boundaries there are to enforce.
	namespaces int

	// rename is what the rename fix knows beyond the references above. It is
	// created on first use, since most passes offer no rename.
	rename *renameState
}

// collectFiles resolves each file's namespace. Generated files are excluded
// entirely: they are neither checked nor treated as reference sites, since a
// violation in generated code is not something the author can act on.
func collectFiles(pass *analysis.Pass, opts Options) *collection {
	c := &collection{
		byFile: make(map[*ast.File]*fileInfo),
		byObj:  make(map[types.Object]*target),
		refs:   make(map[types.Object][]ref),
		idents: make(map[types.Object][]*ast.Ident),

		ignores: make(map[token.Pos]*ignoreSite), scopes: make(map[token.Pos]*scopeSite),
		consumed: make(map[*ast.CommentGroup]bool),
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
			// is what makes "unlabeled" name exactly one unit.
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
		subject := !reachableOutside(d.Name.Name, nil)
		sc, boundBy := c.bind(opts, subject, dir, directive.Decl{}, fi.scope)
		c.add(&target{
			obj: obj, ident: d.Name, kind: kindFunc, file: fi,
			ownerNS: fi.ns, ownerKey: fi.key(), ownerFile: fi, dir: dir, anchor: d.Pos(),
			scope:      sc,
			boundBy:    boundBy,
			subject:    subject,
			renameable: true,
		})
		return
	}

	// A method is an ordinary top-level declaration that happens to name a
	// receiver: the file it is written in gives it its namespace, exactly as
	// for a func, and its type reaches neither its scope nor its ignores. The
	// receiver is still read, for the name a diagnostic prints and because an
	// exported method of an unexported type is reachable from nobody.
	ownerObj := c.receiver(pass, obj)
	owner := ""
	if ownerObj != nil {
		owner = ownerObj.Name()
	}
	subject := !reachableOutside(d.Name.Name, ownerObj)
	sc, boundBy := c.bind(opts, subject, dir, directive.Decl{}, fi.scope)
	c.add(&target{
		obj: obj, ident: d.Name, kind: kindMethod, file: fi,
		owner: owner, ownerObj: ownerObj,
		ownerNS: fi.ns, ownerKey: fi.key(), ownerFile: fi,
		dir: dir, anchor: d.Pos(),
		scope:   sc,
		boundBy: boundBy,
		subject: subject,
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
		attached := append(attached(d.Specs[0]), attached(d.Specs[len(d.Specs)-1])...)
		outer = outer.Merge(c.parseDecl(fi.looseTrailing(pass.Fset, d, attached...)...))
	}

	for _, spec := range d.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			dir := outer.Merge(c.parseDecl(c.specGroups(pass, fi, spec)...))
			anchor := d.Pos()
			if grouped {
				anchor = spec.Pos()
			}
			if obj, ok := pass.TypesInfo.Defs[spec.Name]; ok && spec.Name.Name != "_" {
				subject := !reachableOutside(spec.Name.Name, nil)
				sc, boundBy := c.bind(opts, subject, dir, directive.Decl{}, fi.scope)
				c.add(&target{
					obj: obj, ident: spec.Name, kind: kindType, file: fi,
					ownerNS: fi.ns, ownerKey: fi.key(), ownerFile: fi, dir: dir, anchor: anchor,
					scope:      sc,
					boundBy:    boundBy,
					subject:    subject,
					renameable: true,
				})
			}
			c.addFields(pass, opts, fi, spec, pass.TypesInfo.Defs[spec.Name], dir)

		case *ast.ValueSpec:
			dir := outer.Merge(c.parseDecl(c.specGroups(pass, fi, spec)...))
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
				subject := !reachableOutside(name.Name, nil)
				sc, boundBy := c.bind(opts, subject, dir, directive.Decl{}, fi.scope)
				c.add(&target{
					obj: obj, ident: name, kind: k, file: fi,
					ownerNS: fi.ns, ownerKey: fi.key(), ownerFile: fi, dir: dir, anchor: anchor,
					scope:      sc,
					boundBy:    boundBy,
					subject:    subject,
					renameable: true,
				})
			}
		}
	}
}

// addFields registers the fields of a named struct type.
//
// A field is written inside its type's declaration, so the file it is in is the
// type's: that is where it is, not a binding chosen for it. The type's directive
// therefore contains the field the way a var (...) block contains its specs, and
// is consulted before the file level.
func (c *collection) addFields(pass *analysis.Pass, opts Options, fi *fileInfo, spec *ast.TypeSpec, ownerObj types.Object, container directive.Decl) {
	st, ok := spec.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return
	}
	for _, field := range st.Fields.List {
		// Parsed before the embedded-field check so that a directive on an
		// embedded field is accounted for: it reaches no checked declaration
		// and is reported unused, rather than dropped.
		dir := c.parseDecl(field.Doc, field.Comment)
		// Embedded fields take their name from the embedded type; renaming or
		// hiding them is not meaningful here.
		if len(field.Names) == 0 {
			continue
		}
		for _, name := range field.Names {
			obj, ok := pass.TypesInfo.Defs[name]
			if !ok || name.Name == "_" {
				continue
			}
			subject := !reachableOutside(name.Name, ownerObj)
			sc, boundBy := c.bind(opts, subject, dir, container, fi.scope)
			c.add(&target{
				obj: obj, ident: name, kind: kindField, file: fi,
				owner: spec.Name.Name, ownerObj: ownerObj,
				ownerNS: fi.ns, ownerKey: fi.key(), ownerFile: fi, dir: dir,
				anchor:  field.Pos(),
				scope:   sc,
				boundBy: boundBy,
				subject: subject,
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

// collectRefs records every ident naming a tracked object, along with the file
// it appears in.
//
// An ident can play both roles at once: an embedded field's ident defines the
// field and uses the type, so Defs and Uses are consulted independently rather
// than one shadowing the other. Returning after a hit in Defs would drop the
// type use, and with it both a rename edit and a boundary diagnostic.
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

// origin maps an instantiated field or method back to the object declared in
// the source. go/types records the instantiated object in Uses for a selection
// on a generic type — List[int].items, and List[T].items inside List's own
// methods — while byObj is keyed by the declaration, so without this every
// member of a generic type would go unchecked. Anything else is returned as is.
func origin(obj types.Object) types.Object {
	switch o := obj.(type) {
	case *types.Var:
		return o.Origin()
	case *types.Func:
		return o.Origin()
	}
	return obj
}

// embeddedTypeName returns the type name an embedded field is spelled with,
// and nil for any other object. Such a field has no name of its own: u.count
// is written with the type's name, so a rename of the type has to rewrite the
// selection as well as the embedding. The alias is deliberately not resolved:
// a field embedding an alias is spelled with the alias's name.
func embeddedTypeName(obj types.Object) types.Object {
	v, ok := obj.(*types.Var)
	if !ok || !v.Embedded() {
		return nil
	}
	t := v.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	switch t := t.(type) {
	case *types.Named:
		return t.Obj()
	case *types.Alias:
		return t.Obj()
	}
	return nil
}

// fileAt finds the file a position falls in. A directive problem is not
// attached to any declaration — a stray comment belongs to nothing — so the
// file is the only level that can answer for it.
func (c *collection) fileAt(pass *analysis.Pass, pos token.Pos) *fileInfo {
	path := pass.Fset.Position(pos).Filename
	for _, fi := range c.files {
		if fi.path == path {
			return fi
		}
	}
	return nil
}

// silencesFile reports whether the file stands r down for everything it holds,
// and marks the ignore that did it used.
//
// The marking is what keeps an ignore written for a directive problem from
// being reported as unused itself: it silences a report that is not attached to
// any declaration, so the per-target accounting never sees it work.
func (c *collection) silencesFile(fi *fileInfo, r rule.Rule) bool {
	if fi == nil {
		return false
	}
	hit := false
	for _, ig := range fi.ignores {
		if ig.Covers(r) {
			c.site(ig).used = true
			hit = true
		}
	}
	return hit
}
