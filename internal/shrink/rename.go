package shrink

import (
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/namespace"
)

// renameEdits returns the edits unexporting c, or why they are withheld.
//
// Nothing outside the package names c by the time this is asked, so every
// identifier to rewrite is in its own package. The guards are the analyzer's
// renameSafe, asked of every variant of the package, since a test variant
// declares and resolves names the ordinary one does not: the new name must be
// free in package scope, in every file scope and at every identifier, must be
// no keyword or predeclared name, and must not be one another fix of this run
// has claimed. A method or field asks instead whether the new name is free on
// its type and on every type that promotes it.
//
//declscope:package // the core offers the fix it returns
func (r *run) renameEdits(c *candidate) ([]Edit, string) {
	newName := namespace.Unexported(c.obj.Name())
	variants := r.mod.byPath[c.pkg.PkgPath]
	switch {
	case newName == c.obj.Name() || newName == "_" || token.IsKeyword(newName):
		return nil, "the unexported name is not an identifier"
	case types.Universe.Lookup(newName) != nil:
		return nil, "the unexported name is predeclared"
	case !c.kind.member() && (newName == "init" || newName == "main"):
		return nil, "the unexported name means something to the toolchain"
	case loadNamedInOwnExcluded(r.mod.excluded, c, newName):
		return nil, "a build-excluded file of its package writes the unexported name"
	}
	// A name another fix of this run claims reads the same as one already
	// taken: after that fix it is, and a report the run keeps must read the
	// same once the fixes are applied.
	var reserve, taken string
	if c.kind.member() {
		taken = "the unexported name is taken on its type"
		if !renameMemberFree(r.mod, variants, c, newName) {
			return nil, taken
		}
		reserve = keyOf(r.mod.fset, c.owner) + "." + newName
	} else {
		taken = "the unexported name is taken or would be captured"
		if !renamePackageFree(variants, r.ev.sites[c.key], newName) {
			return nil, taken
		}
		for _, p := range variants {
			if directive.Linknamed(p.Syntax)[c.obj.Name()] {
				return nil, "a //go:linkname or //export names it"
			}
		}
		reserve = c.pkg.PkgPath + "." + newName
	}
	if r.reserved[reserve] {
		return nil, taken
	}
	r.reserved[reserve] = true

	var edits []Edit
	seen := map[token.Pos]bool{}
	for _, s := range r.ev.sites[c.key] {
		if seen[s.ident.Pos()] {
			continue
		}
		seen[s.ident.Pos()] = true
		p := r.mod.fset.Position(s.ident.Pos())
		edits = append(edits, Edit{Filename: p.Filename, Start: p.Offset, End: p.Offset + len(s.ident.Name), NewText: newName})
	}
	return edits, ""
}

// renamePackageFree reports whether newName can replace a package-level name
// in every variant: nothing in package scope, no import in any file, and no
// local, parameter, result or type parameter at any identifier being renamed.
func renamePackageFree(variants []*packages.Package, sites []evidenceSite, newName string) bool {
	for _, p := range variants {
		scope := p.Types.Scope()
		if scope.Lookup(newName) != nil {
			return false
		}
		for i := range scope.NumChildren() {
			if scope.Child(i).Lookup(newName) != nil {
				return false
			}
		}
	}
	for _, s := range sites {
		inner := s.pkg.Types.Scope().Innermost(s.ident.Pos())
		if inner == nil {
			return false
		}
		if _, obj := inner.LookupParent(newName, s.ident.Pos()); obj != nil {
			return false
		}
	}
	return true
}

// renameMemberFree reports whether newName can replace a method or field
// name: no field or method of that name on the owner, none on any type of the
// package that promotes the member, and no interface of the package asking
// for it. The last is a behavior change rather than a compile error: a type
// that gains method m starts satisfying interface{ m() } in an assertion.
func renameMemberFree(m *loadedModule, variants []*packages.Package, c *candidate, newName string) bool {
	for _, p := range variants {
		if found, _, _ := types.LookupFieldOrMethod(c.owner.Type(), true, p.Types, newName); found != nil {
			return false
		}
		for _, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			if iface, ok := tn.Type().Underlying().(*types.Interface); ok {
				if renameInterfaceAsks(iface, newName) {
					return false
				}
				continue
			}
			sel, _, _ := types.LookupFieldOrMethod(tn.Type(), true, p.Types, c.obj.Name())
			if sel == nil || keyOf(m.fset, origin(sel)) != c.key {
				continue
			}
			if found, _, _ := types.LookupFieldOrMethod(tn.Type(), true, p.Types, newName); found != nil {
				return false
			}
		}
		for _, tv := range p.TypesInfo.Types {
			if iface, ok := types.Unalias(tv.Type).(*types.Interface); ok && renameInterfaceAsks(iface, newName) {
				return false
			}
		}
	}
	return true
}

func renameInterfaceAsks(iface *types.Interface, name string) bool {
	return slices.ContainsFunc(slices.Collect(iface.Methods()), func(fn *types.Func) bool { return fn.Name() == name })
}
