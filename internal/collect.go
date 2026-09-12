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
	"github.com/mpyw/declscope/internal/scope"
)

// fileInfo is a source file together with the namespace it belongs to.
type fileInfo struct {
	file *ast.File
	path string
	ns   string
	// explicit records whether ns came from a directive rather than the file
	// name, which changes the advice given when a namespace has no members.
	explicit bool

	// lineComments indexes every comment group by the line it starts on, so
	// that trailing directives can be found on declarations that carry no
	// comment field of their own, such as ast.FuncDecl.
	lineComments map[int]*ast.CommentGroup

	// ignores stands the whole file outside a naming rule. They apply on top
	// of whatever each declaration says for itself.
	ignores []directive.Ignore
	// ignoresUsed tracks which of them silenced something, so that one that
	// silenced nothing can be reported.
	ignoresUsed []bool
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

// key identifies the namespace for comparison. A file whose name cannot yield
// a valid identifier has no namespace, and must not be treated as sharing one
// with every other such file, so it falls back to its own path.
func (f *fileInfo) key() string {
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
	// ownerNS is the namespace that bounds the declaration. For package-level
	// declarations it is the declaring file's namespace. For methods and
	// fields it is the namespace of the type they belong to, because a member
	// is already namespaced by its receiver and must not be namespaced twice.
	ownerNS  string
	ownerKey string
	// owner names the type a member belongs to, empty for package-level.
	owner string
	// ownerObj is that type's object, through which a member inherits the
	// ignore directives written on its type.
	ownerObj types.Object

	scope scope.Scope
	dir   directive.Decl
	// ignoresUsed tracks which of dir.Ignores silenced something. It lives on
	// the target rather than in the reporting loop because a type's directive
	// can be used up by one of its members.
	ignoresUsed []bool

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

	// namespaces is how many distinct namespaces the package's non-test files
	// declare, which is how many boundaries there are to enforce.
	namespaces int
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
	}
	for _, f := range pass.Files {
		path := pass.Fset.Position(f.Pos()).Filename
		if ast.IsGenerated(f) || opts.Excluded(path) {
			continue
		}
		fi := &fileInfo{file: f, path: path, lineComments: make(map[int]*ast.CommentGroup)}
		fileDir := directive.ParseFile(f)
		fi.ignores = fileDir.Ignores
		fi.ignoresUsed = make([]bool, len(fileDir.Ignores))
		c.problems = append(c.problems, fileDir.Problems...)
		for _, g := range f.Comments {
			line := pass.Fset.Position(g.Pos()).Line
			if _, seen := fi.lineComments[line]; !seen {
				fi.lineComments[line] = g
			}
		}
		if fileDir.HasNamespace {
			fi.ns, fi.explicit = fileDir.Namespace, true
		} else {
			fi.ns = namespace.Of(path)
		}
		c.files = append(c.files, fi)
		c.byFile[f] = fi
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
}

func (c *collection) addFunc(pass *analysis.Pass, opts Options, fi *fileInfo, d *ast.FuncDecl) {
	obj, ok := pass.TypesInfo.Defs[d.Name].(*types.Func)
	if !ok || d.Name.Name == "_" {
		return
	}
	dir := directive.ParseDecl(d.Doc, fi.trailingAt(pass.Fset, d.Pos()))

	if d.Recv == nil {
		// init is not declared in package scope and can never be referenced.
		if d.Name.Name == "init" {
			return
		}
		c.add(&target{
			obj: obj, ident: d.Name, kind: kindFunc, file: fi,
			ownerNS: fi.ns, ownerKey: fi.key(), dir: dir, anchor: d.Pos(),
			scope:      opts.resolve(d.Name.Name, dir),
			renameable: true,
		})
		return
	}

	ownerObj, ownerNS, ownerKey := c.receiver(pass, obj, fi)
	owner := ""
	if ownerObj != nil {
		owner = ownerObj.Name()
	}
	c.add(&target{
		obj: obj, ident: d.Name, kind: kindMethod, file: fi,
		owner: owner, ownerObj: ownerObj, ownerNS: ownerNS, ownerKey: ownerKey,
		dir: dir, anchor: d.Pos(),
		scope: opts.resolve(d.Name.Name, dir),
	})
}

func (c *collection) addGenDecl(pass *analysis.Pass, opts Options, fi *fileInfo, d *ast.GenDecl) {
	if d.Tok == token.IMPORT {
		return
	}
	// A directive on the block applies to every spec; a directive on a spec
	// overrides it.
	outer := directive.ParseDecl(d.Doc)
	grouped := d.Lparen.IsValid()

	for _, spec := range d.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			dir := outer.Merge(directive.ParseDecl(spec.Doc, spec.Comment))
			anchor := d.Pos()
			if grouped {
				anchor = spec.Pos()
			}
			if obj, ok := pass.TypesInfo.Defs[spec.Name]; ok && spec.Name.Name != "_" {
				c.add(&target{
					obj: obj, ident: spec.Name, kind: kindType, file: fi,
					ownerNS: fi.ns, ownerKey: fi.key(), dir: dir, anchor: anchor,
					scope:      opts.resolve(spec.Name.Name, dir),
					renameable: true,
				})
			}
			c.addFields(pass, opts, fi, spec, pass.TypesInfo.Defs[spec.Name])

		case *ast.ValueSpec:
			dir := outer.Merge(directive.ParseDecl(spec.Doc, spec.Comment))
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
				c.add(&target{
					obj: obj, ident: name, kind: k, file: fi,
					ownerNS: fi.ns, ownerKey: fi.key(), dir: dir, anchor: anchor,
					scope:      opts.resolve(name.Name, dir),
					renameable: true,
				})
			}
		}
	}
}

// addFields registers the fields of a named struct type. The owning namespace
// is the one declaring the type, so that a type shared across the package can
// still keep its internals to itself.
func (c *collection) addFields(pass *analysis.Pass, opts Options, fi *fileInfo, spec *ast.TypeSpec, ownerObj types.Object) {
	st, ok := spec.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return
	}
	for _, field := range st.Fields.List {
		// Embedded fields take their name from the embedded type; renaming or
		// hiding them is not meaningful here.
		if len(field.Names) == 0 {
			continue
		}
		dir := directive.ParseDecl(field.Doc, field.Comment)
		for _, name := range field.Names {
			obj, ok := pass.TypesInfo.Defs[name]
			if !ok || name.Name == "_" {
				continue
			}
			c.add(&target{
				obj: obj, ident: name, kind: kindField, file: fi,
				owner: spec.Name.Name, ownerObj: ownerObj,
				ownerNS: fi.ns, ownerKey: fi.key(), dir: dir,
				anchor: field.Pos(),
				scope:  opts.resolve(name.Name, dir),
			})
		}
	}
}

// receiver resolves the type a method belongs to and the namespace of the file
// declaring that type.
func (c *collection) receiver(pass *analysis.Pass, fn *types.Func, fallback *fileInfo) (owner types.Object, ownerNS, ownerKey string) {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil, fallback.ns, fallback.key()
	}
	t := sig.Recv().Type()
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil, fallback.ns, fallback.key()
	}
	owner = named.Obj()
	pos := pass.Fset.Position(owner.Pos())
	for _, fi := range c.files {
		if fi.path == pos.Filename {
			return owner, fi.ns, fi.key()
		}
	}
	return owner, fallback.ns, fallback.key()
}

func (c *collection) add(t *target) {
	t.ignoresUsed = make([]bool, len(t.dir.Ignores))
	c.targets = append(c.targets, t)
	c.byObj[t.obj] = t
	c.problems = append(c.problems, t.dir.Problems...)
}

// collectRefs records every ident naming a tracked object, along with the file
// it appears in.
func (c *collection) collectRefs(pass *analysis.Pass) {
	for _, fi := range c.files {
		ast.Inspect(fi.file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if obj, ok := pass.TypesInfo.Defs[ident]; ok && obj != nil {
				c.idents[obj] = append(c.idents[obj], ident)
				return true
			}
			obj, ok := pass.TypesInfo.Uses[ident]
			if !ok || obj == nil {
				return true
			}
			c.idents[obj] = append(c.idents[obj], ident)
			if _, tracked := c.byObj[obj]; tracked {
				c.refs[obj] = append(c.refs[obj], ref{ident: ident, file: fi})
			}
			return true
		})
	}
}
