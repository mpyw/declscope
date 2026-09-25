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

// renamePlanned is a rename the guards allow, before it claims its name.
//
//declscope:package // the core claims it once the package's judgment settles
type renamePlanned struct {
	// taken is how the report reads when another fix of the run takes the
	// name. It reads the same as the name already being taken, since after
	// that fix it is, and a report the run keeps must read the same once the
	// fixes are applied.
	taken string
	edits []Edit

	// newName is what the declaration becomes.
	//
	//declscope:private // only the claim reads it
	newName string
	// pkgKey claims newName in package scope; empty for a member.
	//
	//declscope:private // only the claim reads it
	pkgKey string
	// member claims newName in every type reaching the declaration; nil for
	// a func, var or const.
	//
	//declscope:private // only the claim reads it
	member *renameClaim
}

// renamePlan returns the rename unexporting c, or why it is withheld.
//
// Nothing outside the package names c by the time this is asked, so every
// identifier to rewrite is in its own package. The guards are the analyzer's
// renameSafe, asked of every variant of the package, since a test variant
// declares and resolves names the ordinary one does not: the new name must be
// free in package scope, in every file scope and at every identifier, and
// must be no keyword or predeclared name. A type's name is also the name of
// every field embedding it, so it must be free on every struct of the package
// that embeds it. A method or field asks instead whether the new name is free
// on its type and on every type that promotes it.
//
// What other fixes of the run claim is not asked here. Which declarations are
// fixed is settled first, and renameClaimIn claims the names afterwards.
//
//declscope:package // the core plans every fix before settling which stand
func (r *run) renamePlan(c *candidate) (*renamePlanned, string) {
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
	plan := &renamePlanned{newName: newName}
	if c.kind.member() {
		plan.taken = "the unexported name is taken on its type"
		if !renameMemberFree(r.mod, facts, c, newName) {
			return nil, plan.taken
		}
	} else {
		plan.taken = "the unexported name is taken or would be captured"
		if !renamePackageFree(variants, r.ev.sites[c.key], newName) {
			return nil, plan.taken
		}
		if c.kind == kindType && !renameEmbeddingFree(r.mod, facts, c, newName) {
			return nil, plan.taken
		}
		if facts.linknamed[c.obj.Name()] {
			return nil, "a //go:linkname or //export names it"
		}
		plan.pkgKey = newName
	}
	// A member, and a type through the fields embedding it, meets another
	// fix's new name only in a type reaching both. A.ID and B.Id both lower
	// to id, and clash exactly in a type embedding A and B: a claim keyed by
	// package would withhold unrelated fixes, and the next run, with nothing
	// claimed, would offer them.
	if c.kind.member() || c.kind == kindType {
		plan.member = &renameClaim{name: newName, reach: renameReach(r.mod, c)}
	}
	// Evidence is read from one variant per import path, so each identifier
	// is a site once.
	for _, s := range r.ev.sites[c.key] {
		p := r.mod.Fset.PositionFor(s.ident.Pos(), false)
		plan.edits = append(plan.edits, Edit{Filename: p.Filename, Start: p.Offset, End: p.Offset + len(s.ident.Name), NewText: newName})
	}
	if e, ok := renameDoc(r.mod.Fset, c.doc, c.obj.Name(), newName); ok {
		plan.edits = append(plan.edits, e)
	}
	return plan, ""
}

// renameClaims is what the fixes of one package have claimed so far.
//
//declscope:package // the core claims through it, one package at a time
type renameClaims struct {
	pkg map[string]bool
	// member is every member name claimed, with the types that hold it.
	//
	//declscope:private // only the claim reads it
	member []renameClaim
}

// renameClaimIn claims plan's name among what claims holds, and reports
// whether it was free. Two exported names can lower to one (Foo and FOO both
// become foo), and the fixes cannot see each other.
//
//declscope:package // the core claims each fix that stands
func (r *run) renameClaimIn(claims *renameClaims, c *candidate, plan *renamePlanned) bool {
	if !r.renameClaimFree(claims, c, plan) {
		return false
	}
	if plan.pkgKey != "" {
		claims.pkg[plan.pkgKey] = true
	}
	if plan.member != nil {
		claims.member = append(claims.member, *plan.member)
	}
	return true
}

// renameClaimFree reports whether plan's name is free among what claims
// holds, claiming nothing. A nil plan was never made, so nothing claims its
// name.
//
//declscope:package // the core asks it of the fixes it withheld
func (r *run) renameClaimFree(claims *renameClaims, c *candidate, plan *renamePlanned) bool {
	if plan == nil {
		return true
	}
	if plan.pkgKey != "" && claims.pkg[plan.pkgKey] {
		return false
	}
	if plan.member != nil {
		facts := r.renameFactsOf(c.pkg.PkgPath)
		for _, t := range slices.Concat(facts.named, facts.structs) {
			if !plan.member.reach(t) {
				continue
			}
			for _, other := range claims.member {
				if other.name == plan.newName && other.reach(t) {
					return false
				}
			}
		}
	}
	return true
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

// renameClaim is a member name a fix gives, with the test for whether a type
// holds it: the member's own type and every type promoting it, or every
// struct embedding the type being renamed.
type renameClaim struct {
	name  string
	reach func(renameTyped) bool
}

// renameReach returns whether a type holds c, under its current name or
// not: for a method or field, the type is c's owner or embeds it at any
// depth, and for a type, it embeds c at any depth. A selection the name
// makes today does not decide it: a shallower member of that name hides c,
// and an ambiguous one finds nothing, yet c takes its new name in that type
// all the same, where it can hide another member or make a selector
// ambiguous. The guards and the claims ask this one test, so a name the
// claims call taken is one the guards of the next run find taken too.
func renameReach(m *module.Module, c *candidate) func(renameTyped) bool {
	match := func(tn *types.TypeName) bool { return tn != nil && keyOf(m.Fset, tn) == c.key }
	if c.kind.member() {
		owner := keyOf(m.Fset, c.owner)
		match = func(tn *types.TypeName) bool {
			if tn == nil {
				return false
			}
			if named, ok := types.Unalias(tn.Type()).(*types.Named); ok {
				tn = named.Origin().Obj()
			}
			return keyOf(m.Fset, tn) == owner
		}
		return func(t renameTyped) bool {
			if named, ok := types.Unalias(t.typ).(*types.Named); ok && match(named.Origin().Obj()) {
				return true
			}
			return renameEmbeds(t.typ, match, map[types.Type]bool{})
		}
	}
	return func(t renameTyped) bool { return renameEmbeds(t.typ, match, map[types.Type]bool{}) }
}

// renameEmbeds reports whether t embeds, at any depth, a field whose type
// name match accepts: the name the field is spelled with, an alias or not.
func renameEmbeds(t types.Type, match func(*types.TypeName) bool, seen map[types.Type]bool) bool {
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		t = named.Origin()
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := range st.NumFields() {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		if match(embeddedTypeName(f)) || renameEmbeds(f.Type(), match, seen) {
			return true
		}
	}
	return false
}

// renameNameIn reports whether newName selects anything on t, ambiguously
// or not.
func renameNameIn(t renameTyped, newName string) bool {
	found, index, _ := types.LookupFieldOrMethod(t.typ, true, t.pkg, newName)
	return found != nil || index != nil
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

// renameEmbeddingFree reports whether newName is free on every type of the
// package that embeds the type at any depth, named or not. The embedded field
// takes the type's new name, and would collide with a field or method already
// called that, or make a selector of it ambiguous.
func renameEmbeddingFree(m *module.Module, facts *renameFacts, c *candidate, newName string) bool {
	holds := renameReach(m, c)
	for _, t := range slices.Concat(facts.structs, facts.named) {
		if holds(t) && renameNameIn(t, newName) {
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
	// Every type holding the member takes the new name, including one where
	// the old name is hidden or ambiguous. An unnamed struct promotes members
	// too: struct{ Named; size int }.
	holds := renameReach(m, c)
	for _, t := range slices.Concat(facts.named, facts.structs) {
		if holds(t) && renameNameIn(t, newName) {
			return false
		}
	}
	return true
}
