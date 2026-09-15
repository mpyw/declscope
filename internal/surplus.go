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
package internal

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/scope"
)

// surplusBook is surplus.go's half of the collection, embedded there.
//
//declscope:package // collection embeds it, and collection lives in the core
type surplusBook struct {
	// surplus is the judgment over every //declscope:package directive in the
	// package. It is computed on first use, so a run that allows surplus, or
	// one that cannot see every file, never pays for it.
	//
	//declscope:private // the type is widened only so the core can embed it
	surplus *surplusState
}

// surplusState maps each fired directive to the target the finding is
// reported through: the first declaration the directive binds, in source
// order. The directive is judged as a whole — one physical comment, however
// many declarations take their scope from it — so one target answers for it.
//
//declscope:private // only the computation below hands it out
type surplusState struct {
	findings map[*target]string
}

// checkSurplus reports whether t is the declaration a fired
// //declscope:package directive is reported through, with the position of the
// directive and the message. report.go words nothing here: the judgment and
// its phrasing both belong to this rule.
//
//declscope:package // report.go turns it into the finding it reports
func (c *collection) checkSurplus(pass *analysis.Pass, opts Options, t *target) (token.Pos, string, bool) {
	if c.surplus == nil {
		c.surplus = c.computeSurplus(pass, opts)
	}
	msg, ok := c.surplus.findings[t]
	if !ok {
		return token.NoPos, "", false
	}
	return t.boundBy.ScopePos, msg, true
}

// computeSurplus judges every //declscope:package directive in the package.
//
// A directive is judged per physical comment, never per declaration: a block's
// directive reaches every spec and a type's reaches its fields, and the advice
// is to delete the comment, which is only safe when nothing that takes its
// scope from it is reached. A declaration that states its own scope does not
// depend on an outer directive, so it neither keeps that directive alive nor
// is reported under it.
func (c *collection) computeSurplus(pass *analysis.Pass, opts Options) *surplusState {
	s := &surplusState{findings: make(map[*target]string)}
	if opts.AllowSurplus || !c.surplusSeesEveryFile(pass) {
		return s
	}

	groups := make(map[token.Pos][]*target)
	for _, t := range c.targets {
		if t.boundBy.HasScope && t.boundBy.Scope == scope.PackageInternal {
			groups[t.boundBy.ScopePos] = append(groups[t.boundBy.ScopePos], t)
		}
	}
	if len(groups) == 0 {
		return s
	}

	reached := c.surplusReached(pass)
	linknamed := surplusLinknamed(pass)
	for _, group := range groups {
		fired := true
		for _, t := range group {
			// An exported name is reached by every importer, which no
			// single-package analysis can see, so it keeps its directive and
			// everything sharing the comment.
			if isExported(t.obj.Name()) || reached[t.obj] || linknamed[t.obj.Name()] || c.surplusSeesUseOutside(t) {
				fired = false
				break
			}
		}
		if !fired {
			continue
		}
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
// either type's fields may be the ones another namespace declared.
func (c *collection) surplusConversions(pass *analysis.Pass, reached map[types.Object]bool) {
	mark := func(t types.Type, fi *fileInfo) {
		st, ok := types.Unalias(t).Underlying().(*types.Struct)
		if !ok {
			return
		}
		for i := range st.NumFields() {
			f := st.Field(i)
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

// surplusLinknamed collects every local name a //go:linkname or //export
// directive binds, in every file of the pass. The directive names the
// declaration as text, from code the analysis does not read.
func surplusLinknamed(pass *analysis.Pass) map[string]bool {
	names := make(map[string]bool)
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
					names[fields[0]] = true
				}
			}
		}
	}
	return names
}
