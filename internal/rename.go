package internal

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// renameBook is rename.go's half of the collection, embedded there.
//
//declscope:shared // collection embeds it, and collection lives in the core
type renameBook struct {
	// rename is what the rename fix knows beyond the references the index
	// holds. It is created on first use, since most passes offer no rename.
	//
	//declscope:private // the type is widened only so the core can embed it
	rename *renameState
}

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

	// asked is every method name an interface of the package asks for, and
	// typed is every type of the package that can hold a member: each
	// non-interface type it defines and each struct type it writes, named or
	// not. Both are gathered on the first method or type a rename checks.
	asked     map[string]bool
	typed     []types.Type
	typedDone bool

	// promoting maps a named type to every type in typed that embeds it at
	// any depth, matched by type through aliases, and embedding maps a type
	// name to every type in typed embedding a field spelled with it. Both are
	// built with typed (indexHolders), so a guard looks a candidate up rather
	// than walks every type of the package for each.
	promoting map[*types.Named][]types.Type
	embedding map[types.Object][]types.Type

	// members holds, for each type a guard has asked about, every field and
	// method name declared on it or on anything it embeds (renameMembersOf).
	members map[types.Type]map[string]bool
}

// renameFix rewrites every ident naming the target. All of them are inside the
// package, so the edits stay within the pass.
//
// The fix is offered only when safeToRename can prove it changes nothing but
// the spelling; the diagnostic is reported either way. Whatever it renames to
// is reserved for the rest of the pass, since a later fix checking the same
// pre-fix state would otherwise find the name still free.
//
//declscope:shared // report.go attaches it to the naming findings
func (c *collection) renameFix(pass *analysis.Pass, t *target, newName, message string) (analysis.SuggestedFix, bool) {
	if newName == t.obj.Name() {
		return analysis.SuggestedFix{}, false
	}
	idents := c.idents[t.obj]
	if len(idents) == 0 {
		return analysis.SuggestedFix{}, false
	}
	if !c.safeToRename(pass, t, newName) {
		return analysis.SuggestedFix{}, false
	}
	c.reserveForRename(newName)
	edits := make([]analysis.TextEdit, 0, len(idents))
	for _, id := range idents {
		edits = append(edits, analysis.TextEdit{Pos: id.Pos(), End: id.End(), NewText: []byte(newName)})
	}
	return analysis.SuggestedFix{
		Message:   fmt.Sprintf("rename %s to %s (%s)", t.obj.Name(), newName, message),
		TextEdits: edits,
	}, true
}

// safeToRename reports whether renaming t to newName can be proven not to change
// what the package computes, which is the only condition under which the fix
// is offered. Every check is conservative: a doubt withholds the fix, never
// the diagnostic.
func (c *collection) safeToRename(pass *analysis.Pass, t *target, newName string) bool {
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
	// A method's name and an embedded field's are selected through types, not
	// scopes, so nothing above sees them.
	if !rs.selectionsFree(pass, t, newName) {
		return false
	}
	// A reference in a file the pass did not collect cannot be rewritten.
	if rs.usedOutside(pass, c)[t.obj] {
		return false
	}
	// A directive names the object as text.
	if c.linknamed(pass)[t.obj.Name()] {
		return false
	}
	// Assembly and cgo name Go symbols from files go/types never reads, so
	// a function an .s file defines or calls would lose its link. Reading
	// them for symbols is rejected: a missed spelling is a broken build.
	if c.nonGoBlocksRename(pass) {
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

// reserveForRename records that a fix emitted in this pass renames something to name.
func (c *collection) reserveForRename(name string) {
	c.renames().reserved[name] = true
}

func (c *collection) renames() *renameState {
	if c.rename == nil {
		c.rename = &renameState{reserved: make(map[string]bool)}
	}
	return c.rename
}

// nonGoBlocksRename reports whether the package holds a source file that is not
// Go, in this build or in another. Assembly for another GOARCH is in
// IgnoredFiles, and names the package's symbols in the build that holds it.
// The answer comes from the folder listing (unseen), so go vet, which leaves
// .h and .syso files out of OtherFiles, and the direct driver, which lists
// them, agree. declscope shrink makes the same test before judging a package.
//
//declscope:shared // surplus.go stands down on the same files, which it cannot read either
func (c *collection) nonGoBlocksRename(pass *analysis.Pass) bool {
	return c.unseen(pass).nonGo
}

// renameBoundByImport reports whether any file scope of the package binds name,
// which is where imports live.
func renameBoundByImport(pkg *types.Scope, name string) bool {
	for child := range pkg.Children() {
		if child.Lookup(name) != nil {
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
	in := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	for fc := range in.Root().Children() {
		if _, collected := c.byFile[fc.Node().(*ast.File)]; collected {
			continue
		}
		for cur := range fc.Preorder((*ast.Ident)(nil)) {
			obj := origin(pass.TypesInfo.ObjectOf(cur.Node().(*ast.Ident)))
			if obj == nil {
				continue
			}
			rs.outside[obj] = true
			if tn := embeddedTypeName(obj); tn != nil {
				rs.outside[tn] = true
			}
		}
	}
	return rs.outside
}

// selectionsFree reports whether renaming t to newName leaves every selector
// of the package selecting what it does now. The rename of a method or of an
// embedded type renames a member of every type that holds it, so the new name
// must be free there: a field or method already called that would not
// compile, hide the renamed member, or be hidden by it, and the last can
// compile while computing something else. A method also renames an entry of
// its type's method set, so no interface of the package may ask for either
// name: the type would stop satisfying one, or start satisfying another, and
// a type assertion would change its answer without any error.
//
// This ports declscope shrink's renameMemberFree and renameEmbeddingFree.
// Only the package's own interfaces are asked, and that is complete here
// where it is not for shrink's exported names: the rename is of an unexported
// name, and an unexported method name is qualified by its package, so no
// interface written in another package can ask for it.
func (rs *renameState) selectionsFree(pass *analysis.Pass, t *target, newName string) bool {
	var holders []types.Type
	switch obj := t.obj.(type) {
	case *types.Func:
		recv := obj.Signature().Recv()
		if recv == nil {
			return true
		}
		rs.gatherTyped(pass)
		// An interface method is asked for by its own interface, so a
		// receiver that names no type is never reached in practice.
		owner := renameNamedOf(recv.Type())
		if rs.asked[obj.Name()] || rs.asked[newName] || owner == nil || rs.selects(owner, pass.Pkg, newName) {
			return false
		}
		// An embedding promotes the method whatever name the field is
		// spelled with, so an alias is resolved.
		holders = rs.promoting[owner]
	case *types.TypeName:
		rs.gatherTyped(pass)
		// The field takes the new name only where it is spelled with this
		// one, so an alias is not resolved.
		holders = rs.embedding[obj]
	default:
		return true
	}
	// Every type holding the member takes the new name, including one where
	// the old name is hidden or ambiguous today.
	for _, typ := range holders {
		if rs.selects(typ, pass.Pkg, newName) {
			return false
		}
	}
	return true
}

// gatherTyped fills asked and typed, once per pass.
func (rs *renameState) gatherTyped(pass *analysis.Pass) {
	if rs.typedDone {
		return
	}
	rs.typedDone = true
	rs.asked = make(map[string]bool)
	ask := func(iface *types.Interface) {
		for fn := range iface.Methods() {
			rs.asked[fn.Name()] = true
		}
	}
	for _, obj := range pass.TypesInfo.Defs {
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		if iface, ok := tn.Type().Underlying().(*types.Interface); ok {
			ask(iface)
			continue
		}
		rs.typed = append(rs.typed, tn.Type())
	}
	// A struct type is recorded once per expression of it, so each is kept
	// once.
	seen := make(map[types.Type]bool)
	for _, tv := range pass.TypesInfo.Types {
		switch typ := types.Unalias(tv.Type).(type) {
		case *types.Interface:
			ask(typ)
		case *types.Struct:
			if !seen[typ] {
				seen[typ] = true
				rs.typed = append(rs.typed, typ)
			}
		}
	}
	rs.indexHolders()
}

// indexHolders indexes typed by the embedded fields each type holds at any
// depth: by the named type behind each field into promoting, and by the name
// it is spelled with into embedding.
func (rs *renameState) indexHolders() {
	rs.promoting = make(map[*types.Named][]types.Type)
	rs.embedding = make(map[types.Object][]types.Type)
	for _, typ := range rs.typed {
		promoted, spelled := map[*types.Named]bool{}, map[types.Object]bool{}
		renameEmbeds(typ, map[types.Type]bool{}, func(f *types.Var) {
			if named := renameNamedOf(f.Type()); named != nil {
				promoted[named] = true
			}
			if tn := embeddedTypeName(f); tn != nil {
				spelled[tn] = true
			}
		})
		for named := range promoted {
			rs.promoting[named] = append(rs.promoting[named], typ)
		}
		for tn := range spelled {
			rs.embedding[tn] = append(rs.embedding[tn], typ)
		}
	}
}

// renameNamedOf returns the generic origin of the named type t denotes, or
// that a pointer to it does, and nil for any other type.
func renameNamedOf(t types.Type) *types.Named {
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := types.Unalias(t).(*types.Named); ok {
		return named.Origin()
	}
	return nil
}

// renameEmbeds calls embed with every field t embeds, at any depth.
func renameEmbeds(t types.Type, seen map[types.Type]bool, embed func(*types.Var)) {
	// An embedded field is a named type or a pointer to one, and a type
	// gathered for the guard is a named type or a struct.
	if named := renameNamedOf(t); named != nil {
		t = named
	}
	if seen[t] {
		return
	}
	seen[t] = true
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for f := range st.Fields() {
		if f.Embedded() {
			embed(f)
			renameEmbeds(f.Type(), seen, embed)
		}
	}
}

// selects reports whether name selects anything on t, ambiguously or not.
// The lookup is asked only when one of t's member names is name, since it
// can find nothing else.
func (rs *renameState) selects(t types.Type, pkg *types.Package, name string) bool {
	if rs.members == nil {
		rs.members = make(map[types.Type]map[string]bool)
	}
	members, ok := rs.members[t]
	if !ok {
		members = make(map[string]bool)
		renameMembersOf(t, members, map[types.Type]bool{})
		rs.members[t] = members
	}
	if !members[name] {
		return false
	}
	found, index, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	return found != nil || index != nil
}

// renameMembersOf adds to names every field and method name declared on t or
// on any type it embeds, at any depth. That is every name
// types.LookupFieldOrMethod can find on t: it looks through one pointer, also
// one a defined type is (reporting only a field there), at the methods of a
// named type, the fields of a struct and the method set of an interface, and
// into each embedded field. An instance has its origin's member names.
func renameMembersOf(t types.Type, names map[string]bool, seen map[types.Type]bool) {
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		t = named.Origin()
	}
	if seen[t] {
		return
	}
	seen[t] = true
	if named, ok := t.(*types.Named); ok {
		for fn := range named.Methods() {
			names[fn.Name()] = true
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		renameMembersOf(u.Elem(), names, seen)
	case *types.Interface:
		for fn := range u.Methods() {
			names[fn.Name()] = true
		}
	case *types.Struct:
		for f := range u.Fields() {
			names[f.Name()] = true
			if f.Embedded() {
				renameMembersOf(f.Type(), names, seen)
			}
		}
	}
}
