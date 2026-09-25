package shrink

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/shrink/module"
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
	variants := r.mod.Variants(c.pkg.PkgPath)
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
	case c.writtenInOwn(r.mod.Excluded, newName):
		return nil, "a build-excluded file of its package writes the unexported name"
	}
	// A name another fix of this run claims reads the same as one already
	// taken: after that fix it is, and a report the run keeps must read the
	// same once the fixes are applied.
	var taken string
	if c.kind.member() {
		taken = "the unexported name is taken on its type"
		if !renameMemberFree(r.mod, facts, c, newName) {
			return nil, taken
		}
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
		key := c.pkg.PkgPath + "." + newName
		if r.reserved[key] {
			return nil, taken
		}
		r.reserved[key] = true
	}
	// A member, and a type through the fields embedding it, meets another
	// fix's new name only in a type reaching both. A.ID and B.Id both lower
	// to id, and clash exactly in a type embedding A and B: a claim keyed by
	// package would withhold unrelated fixes, and the next run, with nothing
	// claimed, would offer them.
	claim := renameClaim{name: newName, reach: renameReach(r.mod, c)}
	for _, t := range slices.Concat(facts.named, facts.structs) {
		if !claim.reach(t) {
			continue
		}
		for _, other := range r.claims[c.pkg.PkgPath] {
			if other.name == newName && other.reach(t) {
				return nil, taken
			}
		}
	}
	if c.kind.member() || c.kind == kindType {
		r.claims[c.pkg.PkgPath] = append(r.claims[c.pkg.PkgPath], claim)
	}

	// Evidence is read from one variant per import path, so each identifier
	// is a site once.
	var edits []Edit
	for _, s := range r.ev.sites[c.key] {
		p := r.mod.Fset.PositionFor(s.ident.Pos(), false)
		edits = append(edits, Edit{Filename: p.Filename, Start: p.Offset, End: p.Offset + len(s.ident.Name), NewText: newName})
	}
	if e, ok := renameDoc(r.mod.Fset, c.doc, c.obj.Name(), newName); ok {
		edits = append(edits, e)
	}
	return edits, ""
}

// renameDoc rewrites the name a doc comment opens with, the way Go writes
// one: "// Name ...", or "// A Name ...", "// An Name ...", "// The Name ...".
// Left alone, the comment would describe a name that no longer exists. The
// rest of the comment, and any other mention of the name, is prose and is
// not touched.
func renameDoc(fset *token.FileSet, doc *ast.CommentGroup, oldName, newName string) (Edit, bool) {
	if doc == nil || len(doc.List) == 0 {
		return Edit{}, false
	}
	c := doc.List[0]
	body, ok := strings.CutPrefix(c.Text, "//")
	if !ok {
		return Edit{}, false
	}
	offset := 2
	if rest, ok := strings.CutPrefix(body, " "); ok {
		body, offset = rest, offset+1
	}
	for _, article := range []string{"A ", "An ", "The "} {
		if rest, ok := strings.CutPrefix(body, article); ok && strings.HasPrefix(rest, oldName) {
			body, offset = rest, offset+len(article)
			break
		}
	}
	rest, ok := strings.CutPrefix(body, oldName)
	if !ok {
		return Edit{}, false
	}
	// The name must end where a word does: "Loader" does not open with "Load".
	if r, _ := utf8.DecodeRuneInString(rest); rest != "" && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
		return Edit{}, false
	}
	p := fset.PositionFor(c.Pos(), false)
	return Edit{Filename: p.Filename, Start: p.Offset + offset, End: p.Offset + offset + len(oldName), NewText: newName}, true
}

// renameClaim is a member name a fix of this run gives, with the test for
// whether a type holds it: the member's own type and every type promoting it,
// or every struct embedding the type being renamed.
//
//declscope:package // run keeps every claim of the run
type renameClaim struct {
	//declscope:private // the core holds the claims, not what is in them
	name string
	//declscope:private // the core holds the claims, not what is in them
	reach func(renameTyped) bool
}

// renameReach returns whether a type holds c under its current name. For a
// method or field that is a selection of it, and for a type an embedded
// field named by it. An ambiguous selection counts, since c may be one of the
// candidates it is ambiguous between.
func renameReach(m *module.Module, c *candidate) func(renameTyped) bool {
	return func(t renameTyped) bool {
		obj, index, _ := types.LookupFieldOrMethod(t.typ, true, t.pkg, c.obj.Name())
		if obj == nil {
			return index != nil
		}
		if c.kind == kindType {
			tn := embeddedTypeName(obj)
			return tn != nil && keyOf(m.Fset, tn) == c.key
		}
		return keyOf(m.Fset, origin(obj)) == c.key
	}
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
	for _, p := range r.mod.Variants(path) {
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
func renameEmbeddingFree(m *module.Module, facts *renameFacts, c *candidate, newName string) bool {
	embeds := func(t types.Type) bool {
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		for i := range st.NumFields() {
			if tn := embeddedTypeName(st.Field(i)); tn != nil && keyOf(m.Fset, tn) == c.key {
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
func renameMemberFree(m *module.Module, facts *renameFacts, c *candidate, newName string) bool {
	if facts.asked[newName] {
		return false
	}
	for _, p := range m.Variants(c.pkg.PkgPath) {
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
		if !ambiguous && (sel == nil || keyOf(m.Fset, origin(sel)) != c.key) {
			continue
		}
		if found, _, _ := types.LookupFieldOrMethod(t.typ, true, t.pkg, newName); found != nil {
			return false
		}
	}
	return true
}
