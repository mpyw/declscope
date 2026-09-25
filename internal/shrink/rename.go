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
// has claimed. A type's name is also the name of every field embedding it, so
// it must be free on every struct of the package that embeds it. A method or
// field asks instead whether the new name is free on its type and on every
// type that promotes it.
//
//declscope:package // the core offers the fix it returns
func (r *run) renameEdits(c *candidate) ([]Edit, string) {
	newName, spelled := namespace.Unexported(c.obj.Name())
	variants := r.mod.byPath[c.pkg.PkgPath]
	facts := r.renameFactsOf(c.pkg.PkgPath)
	switch {
	case !spelled:
		return nil, "the name has no unexported spelling Go would use"
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
	//
	// Members share one reservation per package, not one per type. A.ID and
	// B.Id both lower to id, and a type embedding A and B would then hold an
	// ambiguous selector. A type claims the member name as well as the
	// package name, since an embedded field takes the type's name.
	member := c.pkg.PkgPath + "#" + newName
	var reserve []string
	var taken string
	if c.kind.member() {
		taken = "the unexported name is taken on its type"
		if !renameMemberFree(r.mod, facts, c, newName) {
			return nil, taken
		}
		reserve = []string{member}
	} else {
		taken = "the unexported name is taken or would be captured"
		if !renamePackageFree(variants, r.ev.sites[c.key], newName) {
			return nil, taken
		}
		if c.kind == kindType && !renameEmbeddingFree(r.mod, facts, c, newName) {
			return nil, taken
		}
		if facts.linknamed[c.obj.Name()] {
			return nil, "a //go:linkname or //export names it"
		}
		reserve = []string{c.pkg.PkgPath + "." + newName}
		if c.kind == kindType {
			reserve = append(reserve, member)
		}
	}
	for _, key := range reserve {
		if r.reserved[key] {
			return nil, taken
		}
	}
	for _, key := range reserve {
		r.reserved[key] = true
	}

	var edits []Edit
	seen := map[token.Pos]bool{}
	for _, s := range r.ev.sites[c.key] {
		if seen[s.ident.Pos()] {
			continue
		}
		seen[s.ident.Pos()] = true
		p := r.mod.fset.PositionFor(s.ident.Pos(), false)
		edits = append(edits, Edit{Filename: p.Filename, Start: p.Offset, End: p.Offset + len(s.ident.Name), NewText: newName})
	}
	return edits, ""
}

// renameFacts is what the guards ask of one package, gathered once over all
// of its variants rather than once per candidate.
//
//declscope:package // run caches one per package
type renameFacts struct {
	// linknamed is every local name a //go:linkname or //export binds.
	//
	//declscope:private // the core holds the cache, not what is in it
	linknamed map[string]bool
	// asked is every method name an interface of the package asks for.
	//
	//declscope:private // the core holds the cache, not what is in it
	asked map[string]bool
	// named is every non-interface type name the package defines, with the
	// variant that resolves it.
	//
	//declscope:private // the core holds the cache, not what is in it
	named []renameTyped
	// structs is every struct type the package writes, named or not.
	//
	//declscope:private // the core holds the cache, not what is in it
	structs []renameTyped
}

// renameTyped is a type in the variant whose scopes resolve it.
type renameTyped struct {
	pkg *types.Package
	typ types.Type
}

func (r *run) renameFactsOf(path string) *renameFacts {
	if f, ok := r.facts[path]; ok {
		return f
	}
	f := &renameFacts{linknamed: map[string]bool{}, asked: map[string]bool{}}
	for _, p := range r.mod.byPath[path] {
		for name := range directive.Linknamed(p.Syntax) {
			f.linknamed[name] = true
		}
		for _, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			if iface, ok := tn.Type().Underlying().(*types.Interface); ok {
				renameAsk(f.asked, iface)
				continue
			}
			f.named = append(f.named, renameTyped{p.Types, tn.Type()})
		}
		for _, tv := range p.TypesInfo.Types {
			switch t := types.Unalias(tv.Type).(type) {
			case *types.Interface:
				renameAsk(f.asked, t)
			case *types.Struct:
				f.structs = append(f.structs, renameTyped{p.Types, t})
			}
		}
	}
	r.facts[path] = f
	return f
}

func renameAsk(asked map[string]bool, iface *types.Interface) {
	for fn := range iface.Methods() {
		asked[fn.Name()] = true
	}
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

// renameEmbeddingFree reports whether newName is free on every struct of the
// package that embeds the type, and on every named type declared with one.
// The embedded field takes the type's new name, and would collide with a
// field or method already called that, at any depth.
func renameEmbeddingFree(m *loadedModule, facts *renameFacts, c *candidate, newName string) bool {
	embeds := func(t types.Type) bool {
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		for i := range st.NumFields() {
			if tn := embeddedTypeName(st.Field(i)); tn != nil && keyOf(m.fset, tn) == c.key {
				return true
			}
		}
		return false
	}
	for _, t := range slices.Concat(facts.structs, facts.named) {
		if !embeds(t.typ) {
			continue
		}
		if found, _, _ := types.LookupFieldOrMethod(t.typ, true, t.pkg, newName); found != nil {
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
func renameMemberFree(m *loadedModule, facts *renameFacts, c *candidate, newName string) bool {
	if facts.asked[newName] {
		return false
	}
	for _, p := range m.byPath[c.pkg.PkgPath] {
		if found, _, _ := types.LookupFieldOrMethod(c.owner.Type(), true, p.Types, newName); found != nil {
			return false
		}
	}
	// An unnamed struct promotes members too: struct{ Named; size int }.
	for _, t := range slices.Concat(facts.named, facts.structs) {
		// An ambiguous old name (T.F and U.F both embedded) finds nothing,
		// yet the member may be one of the two, and the new name can make a
		// selector that resolves today ambiguous. Such a type is checked too.
		sel, index, _ := types.LookupFieldOrMethod(t.typ, true, t.pkg, c.obj.Name())
		ambiguous := sel == nil && index != nil
		if !ambiguous && (sel == nil || keyOf(m.fset, origin(sel)) != c.key) {
			continue
		}
		if found, _, _ := types.LookupFieldOrMethod(t.typ, true, t.pkg, newName); found != nil {
			return false
		}
	}
	return true
}
