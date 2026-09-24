// surplus.go implements the surplus rule: a //declscope:package directive is
// reported when declscope can see no use of what it widens from another
// namespace. spec/surplus.fsl is the model; NeverReportsReachable is the
// property everything here serves.
//
// The rule is unlike the others in that it concludes from an absence. boundary
// needs a use to find and qualify reads a name, but this rule reports because
// it saw nothing — so every way a declaration can be reached without its name
// being spelled is a way for the rule to be wrong, and being wrong is
// expensive: the advice is to delete a directive, and deleting one that was
// holding something up breaks a package the analysis never saw. Every check
// below therefore silences the rule on a doubt, and there is no fix.
//
// Reach the reference index cannot see, and what answers for it:
//
//   - another namespace spells the name        -> c.refs (usedOutside)
//   - a struct conversion writes fields pairwise, spelling none of them
//   - a method set satisfies an interface contract in the package (satisfies)
//   - an exported type carries an unexported method out of the package, where
//     an importer completes the satisfaction (carrierExposed)
//   - //go:linkname or //export names the declaration as text (linkname)
//   - generated files, cgo and assembly hold reference sites the analysis
//     never reads (opaqueSource) — the rule switches off for the package
//
// seesAllFiles is separate: with -test=false, in the non-test variant of a
// package with in-package tests, or with build-excluded files, some file that
// may hold the use was never read, and the rule switches off rather than guess.
//
// Under rules.surplus: strict the rule also judges one declaration at a time.
// A directive stays quiet as a whole when any one thing under it is reached,
// so a file-level //declscope:package over one helper another file calls and
// one it does not says nothing about the second. strict reports that one, on
// the same evidence, and offers //declscope:private above it. That fix cannot
// break a build: a directive is a comment. What it could do is turn a use
// nobody saw into a boundary report somewhere else, which is the harm the
// checks above exist to prevent, so they are the same checks.
// spec/surplus_strict.fsl is the model for that half.
package internal

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// surplusBook is surplus.go's half of the collection, embedded there.
//
//declscope:package // collection embeds it, and collection lives in the core
type surplusBook struct {
	// surplus is the judgment over every //declscope:package directive in the
	// package. It is computed on first use, so a run that asks nothing of this
	// rule, or one that cannot see every file, never pays for it.
	//
	//declscope:private // the type is widened only so the core can embed it
	surplus *surplusState

	// surplusDeclarations is the strict half of the judgment: each
	// declaration reported on its own, with whether its diagnostic carries the
	// fix. Computed on first use, and only under strict.
	//
	//declscope:private // the type is widened only so the core can embed it
	surplusDeclarations map[*target]surplusDeclaration
}

// surplusDeclaration is one strict finding.
type surplusDeclaration struct {
	msg   string
	fixed bool
	// settledBy is the reported type whose fix narrows this member too. The
	// member is reported only when that type's finding is not, which is
	// report.go's to decide: whether the baseline absorbs it is known there.
	settledBy *target
}

// surplusState is the judgment and the evidence it was made from.
//
// findings maps each fired directive to the target the finding is reported
// through: the first declaration the directive binds, in source order. The
// directive is judged as a whole — one physical comment, however many
// declarations take their scope from it — so one target answers for it.
//
// The rest is kept because strict asks the same questions of one
// declaration: fired says which directives the judgment condemned, and
// reached and linknamed are the evidence that no name index holds. The
// evidence is gathered on the first question, never before: a package with
// no //declscope:package asks none under loose, and interface satisfaction
// is the costly part of it.
type surplusState struct {
	pass      *analysis.Pass
	findings  map[*target]string
	fired     map[token.Pos]bool
	gathered  bool
	reached   map[types.Object]bool
	linknamed map[string]bool
	// blind records that this pass does not read every file, so that nothing
	// above was computed and nothing may be concluded from it.
	blind bool
}

// checkSurplus reports whether t is the declaration a fired
// //declscope:package directive is reported through, with the position of the
// directive and the message. report.go words nothing here: the judgment and
// its phrasing both belong to this rule.
//
//declscope:package // report.go turns it into the finding it reports
func (c *collection) checkSurplus(pass *analysis.Pass, opts Options, t *target) (token.Pos, string, bool) {
	if !opts.Surplus.Reports() {
		return token.NoPos, "", false
	}
	msg, ok := c.surplusJudged(pass).findings[t]
	if !ok {
		return token.NoPos, "", false
	}
	return t.boundBy.ScopePos, msg, true
}

// surplusJudged computes the judgment once per pass. It reads no
// configuration: the mode decides what is reported, never what is true, and
// the strict half is built on this one whatever the mode.
func (c *collection) surplusJudged(pass *analysis.Pass) *surplusState {
	if c.surplus == nil {
		c.surplus = c.computeSurplus(pass)
	}
	return c.surplus
}

// reachesOutside reports whether t is reached from another namespace by any
// path the rule counts, exportedness aside: another namespace spells it,
// something reaches it without spelling it, or a directive names it as text.
// It is never asked of a blind state: a pass that does not read every file
// cannot rule anything out, and every caller returns before asking.
func (s *surplusState) reachesOutside(c *collection, t *target) bool {
	if !s.gathered {
		s.gathered = true
		s.reached = c.surplusReached(s.pass)
		s.linknamed = directive.Linknamed(s.pass.Files)
	}
	return s.reached[t.obj] || s.linknamed[t.obj.Name()] || c.surplusSeesUseOutside(t)
}

// computeSurplus judges every //declscope:package directive in the package.
//
// A directive is judged per physical comment, never per declaration: a block's
// directive reaches every spec and a type's reaches its fields, and the advice
// is to delete the comment, which is only safe when nothing that takes its
// scope from it is reached. A declaration that states its own scope does not
// depend on an outer directive, so it neither keeps that directive alive nor
// is reported under it.
func (c *collection) computeSurplus(pass *analysis.Pass) *surplusState {
	s := &surplusState{pass: pass, findings: make(map[*target]string), fired: make(map[token.Pos]bool)}
	if !c.surplusSeesEveryFile(pass) {
		s.blind = true
		return s
	}

	groups := make(map[token.Pos][]*target)
	for _, t := range c.targets {
		if t.boundBy.HasScope && t.boundBy.Scope == scope.PackageInternal {
			groups[t.boundBy.ScopePos] = append(groups[t.boundBy.ScopePos], t)
		}
	}
	for pos, group := range groups {
		fired := true
		for _, t := range group {
			// An exported name is reached by every importer, which no
			// single-package analysis can see, so it keeps its directive and
			// everything sharing the comment.
			if isExported(t.obj.Name()) || s.reachesOutside(c, t) {
				fired = false
				break
			}
		}
		if !fired {
			continue
		}
		s.fired[pos] = true
		// The finding is keyed to the first declaration in source order, not in
		// c.targets order: the analysis sorts the targets before reporting,
		// but the baseline regeneration does not, and the two must agree on
		// the name that keysForReport the entry.
		slices.SortFunc(group, func(a, b *target) int {
			return comparePos(pass.Fset, a.ident.Pos(), b.ident.Pos())
		})
		rep := group[0]
		s.findings[rep] = surplusMessage(rep, group)
	}
	return s
}

// surplusMessage words one finding. The wording claims only what the analysis
// did: it saw no use, which is weaker than "unused" — a use it cannot see is
// exactly what the checks above could not rule out.
func surplusMessage(rep *target, group []*target) string {
	directive := scope.PackageInternal.Directive()
	if rep.boundAt == scopesiteLevelFile {
		return "the file's " + directive + ": no use from another namespace is visible to declscope"
	}
	names := make([]string, 0, len(group))
	for _, t := range group {
		names = append(names, t.name())
	}
	return directive + " on " + strings.Join(names, ", ") + ": no use from another namespace is visible to declscope"
}

// surplusSeesEveryFile reports whether this pass reads every reference site the
// package has. Where it does not, the rule switches off for the package: an
// absence read from an incomplete view is not evidence.
//
//   - An in-package _test.go this pass does not collect, or a build-excluded
//     file, may hold the one use (unseen — the same scan the rename guard and
//     the unused-ignore report defer on).
//   - A generated or exclude'd file is in pass.Files but is deliberately not
//     read as a reference site.
//   - cgo and assembly reach declarations from sources the analysis never
//     parses at all.
//
//declscope:package // the survey reports whether the rule was asked at all
func (c *collection) surplusSeesEveryFile(pass *analysis.Pass) bool {
	if u := c.unseen(pass); u.all || len(u.names) > 0 {
		return false
	}
	if len(pass.OtherFiles) > 0 || len(pass.IgnoredFiles) > 0 {
		return false
	}
	for _, f := range pass.Files {
		if _, collected := c.byFile[f]; !collected {
			return false
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == `"C"` {
				return false
			}
		}
	}
	return true
}

// surplusSeesUseOutside reports whether another namespace spells the name — the
// same evidence the boundary rule reads, from the same index, so whatever a
// composite literal without keysForReport or a selection on a generic type counts for
// there counts here.
func (c *collection) surplusSeesUseOutside(t *target) bool {
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.file.key() {
			return true
		}
	}
	return false
}

// surplusReached collects every declaration reached by a path that spells no
// name, so that the index of references cannot hold it.
func (c *collection) surplusReached(pass *analysis.Pass) map[types.Object]bool {
	reached := make(map[types.Object]bool)
	interfaces, satisfiers := surplusTypes(pass)
	surplusSatisfies(pass, reached, interfaces, satisfiers)
	surplusCarried(pass, reached)
	c.surplusConversions(pass, reached)
	return reached
}

// surplusTypes gathers the interfaces of the package and every type that
// could satisfy one.
//
// Neither side may be read from the package scope alone. An anonymous
// interface in a type assertion, an anonymous struct, a type declared inside a
// function and an instantiated generic type never appear there, so both sides
// are collected from every type the AST mentions (TypesInfo.Types, values
// included — the type of a call's result is mentioned by no type expression),
// from every instantiation (TypesInfo.Instances) and from every defined type
// name, where a type parameter contributes its constraint.
func surplusTypes(pass *analysis.Pass) (interfaces []*types.Interface, satisfiers []types.Type) {
	seen := make(map[types.Type]bool)
	add := func(t types.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		satisfiers = append(satisfiers, t)
		if i, ok := t.Underlying().(*types.Interface); ok {
			interfaces = append(interfaces, i)
		}
	}
	for _, tv := range pass.TypesInfo.Types {
		add(tv.Type)
	}
	for _, inst := range pass.TypesInfo.Instances {
		add(inst.Type)
	}
	for _, obj := range pass.TypesInfo.Defs {
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		add(tn.Type())
		if tp, ok := tn.Type().(*types.TypeParam); ok {
			add(tp.Constraint())
		}
	}
	return interfaces, satisfiers
}

// surplusSatisfies marks every method that participates in an interface
// contract of the package: an interface value built from the type reaches the
// method with the interface's name, not the method's, so the reference index
// holds nothing.
//
// The lookup passes addressable=true, because a method promoted from a
// pointer-receiver embedding is otherwise not found. The found method is a
// different object from the interface's own requirement — a type trivially
// implements its own interface, and marking a requirement from itself would
// suppress the rule on every interface method — and it goes through Origin,
// because on an instantiated generic type the found object is not the one the
// source declares.
func surplusSatisfies(pass *analysis.Pass, reached map[types.Object]bool, interfaces []*types.Interface, satisfiers []types.Type) {
	for _, iface := range interfaces {
		if iface.NumMethods() == 0 {
			continue
		}
		for _, t := range satisfiers {
			if !types.Implements(t, iface) && !types.Implements(types.NewPointer(t), iface) {
				continue
			}
			for i := range iface.NumMethods() {
				want := iface.Method(i)
				obj, _, _ := types.LookupFieldOrMethod(t, true, pass.Pkg, want.Name())
				fn, ok := obj.(*types.Func)
				if !ok || fn == want {
					continue
				}
				reached[origin(fn)] = true
			}
		}
	}
}

// surplusCarried marks every unexported method an exported type carries out
// of the package. An importer that embeds the type inherits the method — or,
// for an interface, the requirement — and can complete an interface
// satisfaction this analysis never sees, so such a method keeps its directive.
//
// The carrier must be spellable from outside: an exported type name or alias
// in package scope. A method promoted into it from an unexported embedded type
// travels with it, which is why the whole method set is read rather than the
// declared methods.
func surplusCarried(pass *analysis.Pass, reached map[types.Object]bool) {
	pkgScope := pass.Pkg.Scope()
	for _, name := range pkgScope.Names() {
		if !isExported(name) {
			continue
		}
		tn, ok := pkgScope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		t := types.Unalias(tn.Type())
		for _, ms := range []*types.MethodSet{types.NewMethodSet(t), types.NewMethodSet(types.NewPointer(t))} {
			for i := range ms.Len() {
				fn := ms.At(i).Obj()
				if !isExported(fn.Name()) {
					reached[origin(fn)] = true
				}
			}
		}
	}
}

// surplusConversions marks the unexported fields a struct conversion writes.
// snapshot(user) pairs every field of both types by name and spells none of
// them, so a field reached only this way looks unused to the reference index.
// Both sides are marked: the conversion reads one and writes the other, and
// either type's fields may be the ones another namespace declared. An
// instantiated generic struct holds the instantiated fields, which origin maps
// back to the declared ones byObj is keyed by.
func (c *collection) surplusConversions(pass *analysis.Pass, reached map[types.Object]bool) {
	mark := func(t types.Type, fi *fileInfo) {
		st, ok := types.Unalias(t).Underlying().(*types.Struct)
		if !ok {
			return
		}
		for i := range st.NumFields() {
			f := origin(st.Field(i))
			target, tracked := c.byObj[f]
			if tracked && target.file.key() != fi.key() {
				reached[f] = true
			}
		}
	}
	for _, fi := range c.files {
		ast.Inspect(fi.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			tv, ok := pass.TypesInfo.Types[call.Fun]
			if !ok || !tv.IsType() {
				return true
			}
			src := pass.TypesInfo.Types[call.Args[0]]
			if types.Identical(tv.Type, src.Type) {
				return true
			}
			mark(tv.Type, fi)
			mark(src.Type, fi)
			return true
		})
	}
}

// checkSurplusDeclaration reports whether t is a declaration strict reports on
// its own, with the message, the fix, and the type whose finding settles it
// (see surplusDeclaration.settledBy).
//
//declscope:package // report.go turns it into the finding it reports
func (c *collection) checkSurplusDeclaration(pass *analysis.Pass, opts Options, t *target) (string, *analysis.SuggestedFix, *target, bool) {
	if !opts.Surplus.ReportsDeclarations() {
		return "", nil, nil, false
	}
	if c.surplusDeclarations == nil {
		c.surplusDeclarations = c.computeSurplusDeclarations(pass, opts)
	}
	f, ok := c.surplusDeclarations[t]
	if !ok {
		return "", nil, nil, false
	}
	if !f.fixed {
		return f.msg, nil, f.settledBy, true
	}
	fix := analysis.SuggestedFix{
		Message:   fmt.Sprintf("add %s to %s", scope.Private.Directive(), t.name()),
		TextEdits: []analysis.TextEdit{directiveEditInReport(pass, t, scope.Private, t.doc)},
	}
	return f.msg, &fix, f.settledBy, true
}

// surplusEnclosed reports whether t takes package scope from a directive it
// did not write itself — its type's, its block's, or its file's — and would be
// private without it, under the configuration in force.
//
// Four things each settle it the other way, and none of them is an enclosing
// directive's doing:
//
//   - An exported declaration is package-scoped by exportedness alone.
//   - A declaration that states its own scope answers for itself. A redundant
//     //declscope:package there is the unused rule's report, not this one.
//   - A declaration whose scope comes from defaults.unexported took no
//     directive.
//   - Under defaults.unexported: package it would be package-scoped with no
//     directive at all, so the directive widened nothing.
//
// The last one reads the configuration, which the unused rule's binding
// test under loose deliberately does not. The two questions differ. That test
// decides whether a comment is unused under every configuration, because the
// comment outlives any one of them. This one asks whether the
// directive is what widened the declaration today. The fix binds under every
// configuration regardless: the enclosing directive fixes the scope the
// declaration would otherwise take, so //declscope:private under it differs.
func surplusEnclosed(opts Options, t *target) bool {
	if isExported(t.obj.Name()) || t.scope != scope.PackageInternal || opts.Unexported != scope.Private {
		return false
	}
	// Package scope on an unexported name under a private default came from
	// a directive, so the level is never the default here.
	return t.boundAt == scopesiteLevelContainer || t.boundAt == scopesiteLevelFile ||
		t.boundAt == scopesiteLevelDecl && t.fromBlock
}

// computeSurplusDeclarations judges each declaration under a directive that is
// otherwise in use. A directive nothing needs is loose's finding, reported
// once through the directive, and one finding per declaration under it would
// say the same thing again.
//
// Declarations are judged per anchor, never per name. var a, b and a, b int
// are one entry each, and the only directive that can narrow b is one written
// above it, which narrows a too. So the entry is reported only when every name
// in it is: a report on b alone could be cleared only by splitting the
// declaration, which is the author's call. Each name still gets its own
// diagnostic, so that a baseline or an ignore can name it, and the first
// carries the one fix.
//
// A type is judged with what it contains. Narrowing it narrows every member
// that takes its scope through it, so a type is reported only when each such
// member is unexported and unreached as well, and those members are then not
// reported again: the type's fix settles them.
func (c *collection) computeSurplusDeclarations(pass *analysis.Pass, opts Options) map[*target]surplusDeclaration {
	out := make(map[*target]surplusDeclaration)
	s := c.surplusJudged(pass)
	if s.blind {
		return out
	}
	wide := func(t *target) bool {
		return surplusEnclosed(opts, t) && !s.fired[t.boundBy.ScopePos] &&
			!s.reachesOutside(c, t) && !surplusToolchainNamed(pass, t)
	}

	inheriting := make(map[types.Object][]*target)
	entries := make(map[token.Pos][]*target)
	var anchors []token.Pos
	for _, t := range c.targets {
		if t.contained && t.boundAt != scopesiteLevelDecl {
			inheriting[t.ownerObj] = append(inheriting[t.ownerObj], t)
		}
		if _, seen := entries[t.anchor]; !seen {
			anchors = append(anchors, t.anchor)
		}
		entries[t.anchor] = append(entries[t.anchor], t)
	}
	carriesOnlyQuiet := func(t *target) bool {
		return t.kind != kindType || !slices.ContainsFunc(inheriting[t.obj], func(m *target) bool {
			return isExported(m.obj.Name()) || s.reachesOutside(c, m)
		})
	}

	reported := make(map[*target]bool)
	var groups [][]*target
	for _, a := range anchors {
		names := entries[a]
		if slices.ContainsFunc(names, func(t *target) bool { return !wide(t) || !carriesOnlyQuiet(t) }) {
			continue
		}
		groups = append(groups, names)
		for _, t := range names {
			reported[t] = true
		}
	}

	// Members of a reported type are settled by its fix, and join what the
	// run narrows. They stay findings, marked with the type: a baseline that
	// absorbs the type's finding offers no fix to settle them, and a member
	// added since must then be reported in its own right.
	narrowed := make(map[*target]bool)
	settledBy := make(map[*target]*target)
	for _, names := range groups {
		for _, t := range names {
			narrowed[t] = true
			for _, m := range inheriting[t.obj] {
				narrowed[m] = true
			}
		}
		if owner, ok := c.byObj[names[0].ownerObj]; ok && names[0].contained && reported[owner] {
			for _, t := range names {
				settledBy[t] = owner
			}
		}
	}

	// A directive that decides something today and would decide nothing once
	// everything it widens is narrowed is one the fixes would leave unused,
	// and the unused rule would report what -fix had just written. Its
	// advice is to delete the directive, which no fix does, so what is under
	// it is reported without one. A directive that already decides nothing is
	// already reported, and narrowing under it changes nothing about that.
	boundBefore, boundAfter := make(map[token.Pos]bool), make(map[token.Pos]bool)
	for _, t := range c.targets {
		if !t.decided {
			continue
		}
		boundBefore[t.boundBy.ScopePos] = true
		if !narrowed[t] {
			boundAfter[t.boundBy.ScopePos] = true
		}
	}
	for _, names := range groups {
		slices.SortFunc(names, func(a, b *target) int { return comparePos(pass.Fset, a.ident.Pos(), b.ident.Pos()) })
		pos := names[0].boundBy.ScopePos
		fixed := !boundBefore[pos] || boundAfter[pos]
		for i, t := range names {
			out[t] = surplusDeclaration{msg: surplusDeclarationMessage(t), fixed: fixed && i == 0, settledBy: settledBy[t]}
		}
	}
	return out
}

// surplusToolchainNamed reports whether the toolchain finds the declaration by
// name, so that no directive on it says anything: main in package main.
func surplusToolchainNamed(pass *analysis.Pass, t *target) bool {
	return t.kind == kindFunc && t.obj.Name() == "main" && pass.Pkg.Name() == "main"
}

// surplusDeclarationMessage names the level that supplied the scope, as the
// boundary report does, and claims only what the analysis did: it saw no use.
func surplusDeclarationMessage(t *target) string {
	var from string
	switch t.boundAt {
	case scopesiteLevelFile:
		from = "the file's " + scope.PackageInternal.Directive()
	case scopesiteLevelContainer:
		from = scope.PackageInternal.Directive() + " on " + t.owner
	default:
		from = "the " + scope.PackageInternal.Directive() + " on its block"
	}
	return fmt.Sprintf("%s %s takes package scope from %s, but no use from another namespace is visible to declscope",
		t.kind, t.name(), from)
}

// surplusNarrowedByTypeFix names the members a boundary fix on typ has to
// narrow in the same edit, under strict.
//
// That fix writes //declscope:package on a type that took its private scope
// from defaults.unexported, and the directive reaches every member. A member
// no other namespace reads would then be exactly what strict reports, so the
// run would end with a diagnostic it did not start with. The answer is the one
// the two rules agree on: the type widens, and each such member is narrowed.
//
// The judgment is the one computeSurplusDeclarations would make of the fixed
// source, predicted from this one. The type's new directive decides for the
// type itself, which is unexported, so it can never be left binding nothing,
// and the type crosses, so loose can never condemn it. What is left is the
// per-entry test, and an ignore the author already wrote for this rule, which
// is consulted without being marked used: nothing in this run is silenced by
// it.
//
//declscope:package // report.go's boundary fix on a type asks it
func (c *collection) surplusNarrowedByTypeFix(pass *analysis.Pass, opts Options, typ *target) []*target {
	if !opts.Surplus.ReportsDeclarations() || opts.Unexported != scope.Private {
		return nil
	}
	s := c.surplusJudged(pass)
	if s.blind {
		return nil
	}
	entries := make(map[token.Pos][]*target)
	var anchors []token.Pos
	for _, t := range c.targets {
		if !t.contained || t.ownerObj != typ.obj {
			continue
		}
		if _, seen := entries[t.anchor]; !seen {
			anchors = append(anchors, t.anchor)
		}
		entries[t.anchor] = append(entries[t.anchor], t)
	}
	wide := func(t *target) bool {
		return !isExported(t.obj.Name()) && t.boundAt == scopesiteLevelDefault &&
			!s.reachesOutside(c, t) && !c.ignoreWouldSilence(t, rule.Surplus)
	}
	var out []*target
	for _, a := range anchors {
		names := entries[a]
		if !slices.ContainsFunc(names, func(t *target) bool { return !wide(t) }) {
			slices.SortFunc(names, func(a, b *target) int { return comparePos(pass.Fset, a.ident.Pos(), b.ident.Pos()) })
			out = append(out, names[0])
		}
	}
	slices.SortFunc(out, func(a, b *target) int { return comparePos(pass.Fset, a.ident.Pos(), b.ident.Pos()) })
	return out
}
