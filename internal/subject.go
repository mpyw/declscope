// subject.go is the subject the stages share: the file and declaration
// model, the index built over it, and the problem sink. collect.go fills
// the index, every later stage reads it, and each stage appends what it
// finds to the one problem list, so everything here is deliberately
// package-wide. The file joins the core namespace so the model keeps its
// bare names: in a named namespace every type here would need that
// namespace's prefix, and a collectTarget would spell the builder's name
// into the model that every other file reads.
//
//declscope:core
//declscope:package

package internal

import (
	"cmp"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
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

	// core marks the file as part of the package's core namespace, whose prefix
	// is empty. Several files may carry it and they share the one namespace.
	core bool
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

// nsName spells the namespace for the baseline, which is the one place it has
// to be written down rather than compared.
//
// A namespace derived from a file name is normalized to alphanumerics, and one
// written with //declscope:namespace must be an unexported identifier, so a
// parenthesis can appear in neither. That leaves "(core)" free for the core,
// whose prefix is empty and whose files all share it, and free for the file
// with no stem at all — which has no namespace either, and must not share a
// key with every other such file.
func (f *fileInfo) nsName() string {
	if f.core {
		return "(core)"
	}
	if f.ns != "" {
		return f.ns
	}
	return "(file " + filepath.Base(f.path) + ")"
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

	// file is where the declaration is written, and so the file whose
	// namespace bounds it. A member is written inside its type's declaration,
	// so for a member this is the type's file; a method with a receiver is an
	// ordinary top-level declaration and takes its own file's, like a func.
	// The two can never differ, which is why there is one field and not two.
	file *fileInfo
	// contained marks a member written inside its type's declaration: a struct
	// field, and an interface's method name. The type's directives reach it,
	// the way a var (...) block reaches its specs. A method with a receiver is
	// not contained, however much it looks like a member: it is an ordinary
	// top-level declaration that happens to name one.
	contained bool

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
	// boundAt names which level that was.
	boundAt scopeLevel
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

// methodOwner returns the object of the type a method is declared on, nil
// when the receiver names no type in the package. It fills ownerObj for a
// method, the way the enclosing TypeSpec fills it for a member.
func methodOwner(fn *types.Func) types.Object {
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

// scopeLevel names which level supplied a scope. A diagnostic that inferred it
// from the kind instead would tell a reader to look for a comment that is not
// there: a field takes its type's directive and its file's alike, and only the
// level knows which one decided.
type scopeLevel int

const (
	levelDefault scopeLevel = iota
	levelDecl
	levelContainer
	levelFile
)

// ref is a use of a target from somewhere in the package.
type ref struct {
	ident *ast.Ident
	file  *fileInfo
}

// collection is the working state of one pass. collect.go builds the index
// half, which every later stage reads, so it lives here and is package-wide.
//
// Each stage's own working state is embedded instead, from a struct declared
// in the file that owns it. Embedding rather than nesting keeps the call sites
// spelling c.ignores, while the field itself belongs to ignore.go's namespace
// and is private to it. Reaching one from the wrong stage is a boundary
// crossing, which is the thing this package was split up to be able to say.
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

	// The stage-owned halves. Each is declared in the file that owns it.
	ignoreBook
	scopesiteBook
	collectBook
	renameBook

	// unseenScan is what the package directory holds that this pass does not
	// see: in-package _test.go files under the non-test variant, and files the
	// build configuration excluded. The rename fix and the unused-ignore
	// report both consult it. It is filled on first use.
	unseenScan *unseenFiles

	// namespaces is how many distinct namespaces the package's non-test files
	// declare, which is how many boundaries there are to enforce.
	namespaces int
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

// comparePos orders positions by file name, then by offset within the file.
func comparePos(fset *token.FileSet, a, b token.Pos) int {
	pa, pb := fset.Position(a), fset.Position(b)
	if c := strings.Compare(pa.Filename, pb.Filename); c != 0 {
		return c
	}
	return cmp.Compare(pa.Offset, pb.Offset)
}

// isExported is Go's own rule, not an ASCII approximation of it. name[0] is the
// first *byte*: for Äpfel that is 0xC3, so a byte-range test answers
// "unexported" for a name Go exports — which gave the declaration
// defaults.unexported instead of package scope, reported a boundary on
// published API, and let -fix rename it with in-package edits only, breaking
// every importer.
func isExported(name string) bool { return ast.IsExported(name) }

// unseenFiles is what the package directory holds that this pass does not see.
// Two things put a .go file of this package outside pass.Files: it is an
// in-package _test.go and this is the non-test variant, or the build
// configuration excluded it — a _GOOS suffix, a //go:build line. Either way it
// may declare or name the same identifiers, and this pass can neither see that
// nor rewrite it.
//
// The two are not equally hopeless. The test variant sees every test file and
// decides on its own, and its fix rewrites the non-test files too, so under
// -test (the default) deferring wholesale costs nothing. No variant ever sees
// a build-excluded file, so deferring wholesale there would mean no rename is
// ever offered in a package that carries a _GOOS suffix. Those files are read
// instead for the identifiers they write, and only a rename that disturbs one
// of them is withheld.
type unseenFiles struct {
	// all withholds every rename: an in-package test file this pass does not
	// see, or something in the directory that could not be read or parsed.
	all bool

	// names is every identifier written in an unseen file that was read. A
	// rename is withheld when it takes one of these names away or claims one:
	// the excluded file would otherwise still spell the old name, or would
	// find the new one declared twice.
	names map[string]bool
}

// unseen scans the package directory, once per pass. It lives here rather
// than with the rename guard because two features ask it: the rename fix
// (rename.go) and the unused-ignore report (ignore.go) both defer to the
// pass that sees every file.
//
// A _test.go file is parsed for its package clause alone: an external test
// package (package x_test) declares into its own scope and can only name the
// package's exported identifiers, which are never renamed. Every other file
// whose package clause matches is parsed in full, since its identifiers are
// the point. Files are read from disk rather than through pass.ReadFile, whose
// access policy admits only the files of the pass itself; the config lookup
// already reads the filesystem, so this adds no new assumption about the
// driver. Anything that cannot be read or parsed withholds everything, which
// withholds rather than risks.
func (c *collection) unseen(pass *analysis.Pass) *unseenFiles {
	if c.unseenScan != nil {
		return c.unseenScan
	}
	u := &unseenFiles{names: make(map[string]bool)}
	c.unseenScan = u

	dir, inPass := "", make(map[string]bool)
	for _, f := range pass.Files {
		name := pass.Fset.Position(f.Pos()).Filename
		if name == "" {
			continue
		}
		inPass[name] = true
		if dir == "" {
			dir = filepath.Dir(name)
		}
	}
	if dir == "" {
		return u
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		u.all = true
		return u
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if inPass[p] {
			continue
		}
		// The package clause is read first and decides the rest. A file of
		// another package — a //go:build ignore generator declaring package
		// main is the common one — writes nothing this package can name. In
		// the external test variant that clause check skips every file of the
		// package under test, which is the whole directory.
		f, err := parser.ParseFile(fset, p, nil, parser.PackageClauseOnly)
		if err != nil {
			u.all = true
			continue
		}
		if f.Name.Name != pass.Pkg.Name() {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.go") {
			u.all = true
			continue
		}
		if f, err = parser.ParseFile(fset, p, nil, 0); err != nil {
			u.all = true
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				u.names[id.Name] = true
			}
			return true
		})
	}
	return u
}
