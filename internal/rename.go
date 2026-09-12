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
	// The test variant of the package sees files this pass does not: a
	// declaration or a reference there is invisible here, so nothing this
	// pass can check proves the rename safe. The test variant sees every
	// file and decides on its own, and its fix rewrites the non-test files
	// too, so under -test (the default) nothing is lost by deferring to it.
	return !c.hasUnseenTests(pass)
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
// The same normalisation as collectRefs applies, so a selection through an
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

// hasUnseenTests reports whether the package directory holds in-package
// _test.go files that are not in pass.Files. For the test variant, and for a
// package without tests, there are none.
//
// Only the package clause is parsed: an external test package (package x_test)
// declares into its own scope and can only name the package's exported
// identifiers, which are never renamed. The files are read from disk rather
// than through pass.ReadFile, whose access policy admits only the files of the
// pass itself; the config lookup already reads the filesystem, so this adds no
// new assumption about the driver. A directory that cannot be read is treated
// as holding tests, which withholds rather than risks.
func (c *collection) hasUnseenTests(pass *analysis.Pass) bool {
	if c.unseenTestsDone {
		return c.unseenTests
	}
	c.unseenTestsDone = true

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
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.unseenTests = true
		return true
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if inPass[p] {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.PackageClauseOnly)
		if err != nil || f.Name.Name == pass.Pkg.Name() {
			c.unseenTests = true
			return true
		}
	}
	return false
}
