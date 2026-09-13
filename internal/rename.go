package internal

import (
	"fmt"
	"go/ast"
	"go/types"
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
//
//declscope:package // the collection carries it in its rename field
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

// renameFix rewrites every ident naming the target. All of them are inside the
// package, so the edits stay within the pass.
//
// The fix is offered only when renameSafe can prove it changes nothing but
// the spelling; the diagnostic is reported either way. Whatever it renames to
// is reserved for the rest of the pass, since a later fix checking the same
// pre-fix state would otherwise find the name still free.
//
//declscope:package // report.go attaches it to the naming findings
func (c *collection) renameFix(pass *analysis.Pass, t *target, newName, message string) (analysis.SuggestedFix, bool) {
	if newName == t.obj.Name() {
		return analysis.SuggestedFix{}, false
	}
	idents := c.idents[t.obj]
	if len(idents) == 0 {
		return analysis.SuggestedFix{}, false
	}
	if !c.renameSafe(pass, t, newName) {
		return analysis.SuggestedFix{}, false
	}
	c.reserve(newName)
	edits := make([]analysis.TextEdit, 0, len(idents))
	for _, id := range idents {
		edits = append(edits, analysis.TextEdit{Pos: id.Pos(), End: id.End(), NewText: []byte(newName)})
	}
	return analysis.SuggestedFix{
		Message:   fmt.Sprintf("rename %s to %s (%s)", t.obj.Name(), newName, message),
		TextEdits: edits,
	}, true
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
	if renameBoundByImport(pass.Pkg.Scope(), newName) {
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

// renameBoundByImport reports whether any file scope of the package binds name,
// which is where imports live.
func renameBoundByImport(pkg *types.Scope, name string) bool {
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
