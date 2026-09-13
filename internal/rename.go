//declscope:namespace analyzer

package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// renameState is what the rename fix needs to know beyond the references the
// collection gathers for the diagnostics. A rename is offered only when it is
// provably safe, and proving that takes facts the diagnostics never need:
// which objects are named from files the pass did not collect, which names a
// directive binds as text, which names earlier fixes in the same pass have
// already claimed, and whether the package has test files this pass cannot
// see. Everything but the reservation set is derived lazily, since most passes
// offer no rename at all.
type renameState struct {
	// reserved holds every new name a fix emitted in this pass has claimed.
	// Fixes are generated from one pre-fix state and cannot see each other,
	// so without this two declarations could be renamed to the same name.
	reserved map[string]bool

	// outside marks every object named from a file in pass.Files that the
	// pass did not collect, because it is generated or excluded. A reference
	// there cannot be rewritten, so a rename would leave it dangling.
	outside     map[types.Object]bool
	outsideDone bool

	// directives holds every local name a //go:linkname or //export
	// directive in the package binds. The directive names the object as
	// text, which a rename cannot follow.
	directives     map[string]bool
	directivesDone bool
}

func (c *collection) renames() *renameState {
	if c.rename == nil {
		c.rename = &renameState{reserved: make(map[string]bool)}
	}
	return c.rename
}

// renameSafe reports whether renaming t to newName can be proven not to change
// what the package computes, which is the only condition under which the fix
// is offered. Every check is conservative: a doubt withholds the fix, never
// the diagnostic.
func (c *collection) renameSafe(pass *analysis.Pass, t *target, newName string) bool {
	rs := c.renames()

	// An exported declaration has uses outside the package that this analysis
	// never sees: everything is checked within one package, so the rewrite
	// could not be completed and would break every importer. Unlike the guards
	// below this is not a doubt that a larger pass could settle — no run of
	// declscope can ever see the whole of an exported name's uses — and
	// finishing it by hand is an API change, which is the author's call.
	// The violation is reported either way; only the fix is withheld.
	if isExported(t.obj.Name()) {
		return false
	}
	// Renaming into a name the package already uses would not compile.
	if pass.Pkg.Scope().Lookup(newName) != nil {
		return false
	}
	// A package-level name legitimately shadows a predeclared one, so the
	// renamed references would still resolve. Every other use of the builtin
	// in the package would not: `var len = 3` compiles and breaks len(s).
	if types.Universe.Lookup(newName) != nil {
		return false
	}
	// An import binds its name in file scope, and Go rejects a package-level
	// declaration named like an import in any file of the package, not only
	// in one that references the declaration.
	if fileScopesBind(pass.Pkg.Scope(), newName) {
		return false
	}
	// Go resolves a name from the innermost scope outwards, so a target that
	// is free in package scope can still be bound at a reference site by a
	// local, a parameter, a result or a type parameter, and renaming there
	// would silently retarget the reference: `return fooCount + count` with
	// a parameter fooCount becomes `fooCount + fooCount`, which compiles.
	// LookupParent walks from the innermost scope out to the universe, so at
	// a reference it also covers the same file's imports and the predeclared
	// names; the checks above are kept because they also hold where there is
	// no reference to test, and because an import in another file is not on
	// this path.
	for _, id := range c.idents[t.obj] {
		inner := pass.Pkg.Scope().Innermost(id.Pos())
		if inner == nil {
			return false
		}
		if _, obj := inner.LookupParent(newName, id.Pos()); obj != nil {
			return false
		}
	}
	// A reference in a file the pass did not collect cannot be rewritten.
	if rs.usedOutside(pass, c)[t.obj] {
		return false
	}
	// A directive names the object as text.
	if rs.namedByDirective(pass)[t.obj.Name()] {
		return false
	}
	// Another fix in this pass has already taken the name.
	if rs.reserved[newName] {
		return false
	}
	// A file of this package that this pass does not see may declare or name
	// the target, and the fix does not rewrite it. An in-package test file
	// withholds every rename, since the test variant sees every file and
	// decides on its own. A build-excluded file withholds only a rename that
	// disturbs an identifier it writes: no variant will ever see that file, so
	// withholding wholesale would disable the fix for the whole package.
	u := c.unseen(pass)
	if u.all {
		return false
	}
	return !u.names[t.obj.Name()] && !u.names[newName]
}

// reserve records that a fix emitted in this pass renames something to name.
func (c *collection) reserve(name string) {
	c.renames().reserved[name] = true
}

// fileScopesBind reports whether any file scope of the package binds name,
// which is where imports live.
func fileScopesBind(pkg *types.Scope, name string) bool {
	for i := range pkg.NumChildren() {
		if pkg.Child(i).Lookup(name) != nil {
			return true
		}
	}
	return false
}

// usedOutside returns every object named from a file the pass did not collect.
// The same normalization as collectRefs applies, so a selection through an
// embedded field or on a generic type counts as naming the declaration.
func (rs *renameState) usedOutside(pass *analysis.Pass, c *collection) map[types.Object]bool {
	if rs.outsideDone {
		return rs.outside
	}
	rs.outsideDone = true
	rs.outside = make(map[types.Object]bool)
	for _, f := range pass.Files {
		if _, collected := c.byFile[f]; collected {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			obj := origin(pass.TypesInfo.ObjectOf(ident))
			if obj == nil {
				return true
			}
			rs.outside[obj] = true
			if tn := embeddedTypeName(obj); tn != nil {
				rs.outside[tn] = true
			}
			return true
		})
	}
	return rs.outside
}

// namedByDirective returns every local name bound by a //go:linkname or
// //export directive in any file of the pass, collected or not.
func (rs *renameState) namedByDirective(pass *analysis.Pass) map[string]bool {
	if rs.directivesDone {
		return rs.directives
	}
	rs.directivesDone = true
	rs.directives = make(map[string]bool)
	for _, f := range pass.Files {
		for _, g := range f.Comments {
			for _, cmt := range g.List {
				rest, ok := strings.CutPrefix(cmt.Text, "//go:linkname ")
				if !ok {
					rest, ok = strings.CutPrefix(cmt.Text, "//export ")
				}
				if !ok {
					continue
				}
				if fields := strings.Fields(rest); len(fields) > 0 {
					rs.directives[fields[0]] = true
				}
			}
		}
	}
	return rs.directives
}

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

// unseen scans the package directory, once per pass.
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
