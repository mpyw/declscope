// Package shrink finds the exported declarations of internal packages that
// nothing outside their package uses, and unexports them. It backs declscope
// shrink, and spec/shrink.fsl is its model.
//
// The analyzer cannot answer this question. It sees one package, and an
// importer might use any exported name. Inside internal/ the import rule
// limits the importers to one directory tree, so a run that loads the whole
// module sees every one of them, and the absence of a use becomes evidence.
//
// The report and the fix are held to different standards. The report may be
// wrong where a use is possible but not provable, such as a name a
// build-excluded file writes: the fix is withheld there and the report kept.
// The fix is made only where no use can exist outside the package.
//
// The package is laid out by what each part needs to know. What asks only of
// types or files is a subpackage with its own small vocabulary: module loads
// the module and knows its internal ranges, excluded reads the files the
// build left out, and reach follows what a value lets code reach. What works
// on the model below stays here, one file per stage: candidate.go finds the
// declarations to judge, evidence.go gathers who uses what, rename.go decides
// the fix, and ignore.go accounts for //declscope:ignore overexported.
//
// This file is the core: the model every stage reads, and the judgment that
// drives them. In a named namespace each type here would have to carry that
// namespace's name.
//
//declscope:core
package shrink

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/shrink/excluded"
	"github.com/mpyw/declscope/internal/shrink/module"
	"github.com/mpyw/declscope/internal/shrink/reach"
)

// Finding is one report of declscope shrink.
type Finding struct {
	Pos token.Position
	// Rule is overexported for a declaration, and unused for an ignore
	// naming overexported that silenced nothing.
	Rule rule.Rule
	// Name and Kind describe the declaration. Both are empty for an unused
	// ignore.
	Name, Kind string
	// Package is the import path of the package that declares it.
	Package string
	// TestOnly marks a declaration only external tests use. It keeps its
	// name, since the external test package would stop compiling.
	TestOnly bool
	// Withheld says why no fix is offered, empty when Fix holds one.
	Withheld string
	// Fix renames every identifier naming the declaration, all of them in its
	// own package. It is nil when the fix is withheld.
	Fix []Edit
}

// Edit replaces the bytes [Start, End) of a file.
type Edit struct {
	Filename   string
	Start, End int
	NewText    string
}

// Message renders the finding the way the analyzer renders a diagnostic.
func (f Finding) Message() string {
	if f.Rule == rule.Unused {
		return "unused //declscope:ignore overexported: nothing it covers is reported"
	}
	var msg string
	if f.TestOnly {
		msg = fmt.Sprintf("%s %s is exported, but only the external tests of %s use it", f.Kind, f.Name, f.Package)
	} else {
		msg = fmt.Sprintf("%s %s is exported, but nothing outside %s uses it", f.Kind, f.Name, f.Package)
	}
	if f.Withheld != "" {
		msg += " (no fix: " + f.Withheld + ")"
	}
	return msg
}

// Result is what one run found.
type Result struct {
	Findings []Finding
	// Skipped lists the internal packages the patterns named that were not
	// judged, and why. Saying nothing about them would read as nothing
	// overexported.
	Skipped []Skipped
}

// Skipped is an internal package that was not judged.
type Skipped struct {
	Package, Reason string
}

// Run loads the module containing dir and reports on the packages the
// patterns name, resolved from dir. Every package of the module is loaded
// whatever the patterns say, since an importer outside them still counts.
func Run(dir string, patterns []string) (Result, error) {
	mod, err := module.Load(dir)
	if err != nil {
		return Result{}, err
	}
	wanted, err := module.Wanted(dir, patterns)
	if err != nil {
		return Result{}, err
	}
	ev, err := evidenceCollect(mod)
	if err != nil {
		return Result{}, err
	}
	r := &run{
		mod: mod, ev: ev, reserved: map[string]bool{}, facts: map[string]*renameFacts{},
		claims: map[string][]renameClaim{}, usedIgnores: map[token.Pos]bool{},
	}
	var res Result
	for _, path := range mod.Paths {
		if !wanted[path] {
			continue
		}
		pkgs := mod.Variants(path)
		if why := r.skip(pkgs[0]); why != "" {
			// A package outside internal/ is never judged, and listing every
			// one would bury the packages a reader expected judged.
			if _, internal := module.InternalParent(path); internal {
				res.Skipped = append(res.Skipped, Skipped{Package: path, Reason: why})
			}
			continue
		}
		candidates, siblings := candidatesOf(pkgs[0])
		res.Findings = append(res.Findings, r.judge(pkgs[0], candidates)...)
		res.Findings = append(res.Findings, r.unusedIgnores(pkgs[0], siblings)...)
	}
	slices.SortFunc(res.Findings, func(a, b Finding) int {
		return cmp.Or(strings.Compare(a.Pos.Filename, b.Pos.Filename), cmp.Compare(a.Pos.Offset, b.Pos.Offset))
	})
	return res, nil
}

// Apply writes every fix the findings carry.
func Apply(findings []Finding) error {
	byFile := map[string][]Edit{}
	for _, f := range findings {
		for _, e := range f.Fix {
			byFile[e.Filename] = append(byFile[e.Filename], e)
		}
	}
	names := make([]string, 0, len(byFile))
	for name := range byFile {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		edits := byFile[name]
		// Later edits first, so earlier offsets stay valid. Two findings never
		// edit one identifier, since each renames its own declaration.
		slices.SortFunc(edits, func(a, b Edit) int { return cmp.Compare(b.Start, a.Start) })
		src, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		for _, e := range slices.Compact(edits) {
			src = slices.Concat(src[:e.Start], []byte(e.NewText), src[e.End:])
		}
		info, err := os.Stat(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(name, src, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

// kind is what a candidate declares, for the message and for which evidence
// applies to it.
//
//declscope:package // candidate.go and rename.go classify candidates by it
type kind string

//declscope:package // candidate.go classifies each candidate
const (
	kindFunc   kind = "func"
	kindVar    kind = "var"
	kindConst  kind = "const"
	kindType   kind = "type"
	kindMethod kind = "method"
	kindField  kind = "field"
)

// member reports whether the kind is reached through a value of its owner,
// which is how another module uses one without naming the owner.
//
//declscope:package // rename.go asks it
func (k kind) member() bool { return k == kindMethod || k == kindField }

// candidate is an exported declaration of a judged package.
//
//declscope:package // candidate.go builds it, and every stage reads it
type candidate struct {
	obj  types.Object
	key  string
	kind kind
	// owner is the type a method or field belongs to, nil otherwise.
	owner *types.TypeName
	// pkg is the widest variant of the declaring package: with its in-package
	// tests, when it has any.
	pkg     *packages.Package
	ignores []directive.Ignore
	// testFile marks a declaration of an in-package _test.go file.
	testFile bool
	// tagged marks a field written with a struct tag.
	tagged bool
	// doc is the declaration's own doc comment, whose leading name the
	// rename rewrites too.
	doc *ast.CommentGroup
}

// inOwnPackage reports whether a build-excluded file belongs to the
// candidate's own package: its directory and its package name.
func (c *candidate) inOwnPackage(x *excluded.File) bool {
	var dir string
	for _, f := range slices.Concat(c.pkg.GoFiles, c.pkg.CompiledGoFiles) {
		dir = filepath.Dir(f)
		break
	}
	return x.Dir == dir && x.Package == c.pkg.Types.Name()
}

// qualifiedIn reports whether a build-excluded file of another package
// imports the candidate's package and writes pkg.Name: a use, in a
// configuration this run does not build.
func (c *candidate) qualifiedIn(xs []*excluded.File) bool {
	return slices.ContainsFunc(xs, func(x *excluded.File) bool {
		return !c.inOwnPackage(x) && x.Qualifies(c.pkg.PkgPath, c.pkg.Types.Name(), c.obj.Name())
	})
}

// maybeUsedIn reports whether a build-excluded file of another package may
// use the candidate without pinning it to its package: a method or field
// selected by name on some value, or a name under a dot import.
func (c *candidate) maybeUsedIn(xs []*excluded.File) bool {
	name := c.obj.Name()
	return slices.ContainsFunc(xs, func(x *excluded.File) bool {
		if c.inOwnPackage(x) {
			return false
		}
		return c.kind.member() && x.Selects(name) || x.DotImports(c.pkg.PkgPath) && x.Writes(name)
	})
}

// writtenInOwn reports whether a build-excluded file of the candidate's own
// package writes name. The rename cannot rewrite that file, which would keep
// the old name or declare the new one twice.
//
//declscope:package // rename.go asks it about the new name
func (c *candidate) writtenInOwn(xs []*excluded.File, name string) bool {
	return slices.ContainsFunc(xs, func(x *excluded.File) bool { return c.inOwnPackage(x) && x.Writes(name) })
}

// linkerSet reports whether the candidate is a package-level string
// variable, which `go build -ldflags "-X path.Name=value"` sets by name. A
// build file passes the flag, where go/types never looks, and the linker
// ignores a -X whose name no longer exists, so the rename would break the
// release silently.
func (c *candidate) linkerSet() bool {
	v, ok := c.obj.(*types.Var)
	if !ok || c.kind != kindVar {
		return false
	}
	b, ok := v.Type().Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// run is the state one invocation shares across the packages it judges.
//
//declscope:package // rename.go and ignore.go add methods of their own
type run struct {
	mod *module.Module
	ev  *evidence
	// reserved holds every package-level name a fix in this run has claimed.
	// Two exported names can lower to one (Foo and FOO both become foo), and
	// the fixes cannot see each other.
	reserved map[string]bool
	// facts caches what the rename guards ask of each package.
	facts map[string]*renameFacts
	// claims holds the member names this run's fixes give, per package.
	claims map[string][]renameClaim
	// usedIgnores is every ignore naming overexported that silenced a report.
	usedIgnores map[token.Pos]bool
	// held is every type of the package being judged that a declaration
	// staying exported carries.
	//
	//declscope:private // only the judgment reads it
	held map[string]bool
}

// skip says why a package is not judged at all, or returns "".
//
// Each reason is one way an importer could be unseen or a name could be
// read where the analysis never looks, so nothing is said rather than a
// guess.
func (r *run) skip(p *packages.Package) string {
	switch {
	case p.Name == "main":
		// -buildmode=plugin looks exported symbols of main up by name.
		return "package main"
	case len(p.OtherFiles) > 0 || slices.ContainsFunc(p.IgnoredFiles, func(f string) bool { return !strings.HasSuffix(f, ".go") }):
		// Assembly and cgo name Go symbols from files go/types never reads.
		// Assembly for another GOARCH is in IgnoredFiles, and names them in
		// the build this run does not see.
		return "the package holds non-Go files"
	case !slices.Equal(p.GoFiles, p.CompiledGoFiles):
		return "the package uses cgo"
	}
	_, why := r.mod.Range(p.PkgPath)
	return why
}

// judge reports every candidate of one package.
//
// A type that an exported declaration returns, takes or holds keeps its name
// as long as that declaration stays exported, or the declaration would hand
// out a type its callers cannot name. Which declarations stay exported is
// known only once each is judged, and a type kept exported may carry others,
// so judging repeats until no more types are kept. held only grows, since a
// held type is never reported again, and it is bounded by the package's
// types, so the loop ends. A declaration unexported
// in the same run keeps nothing: it and the type it returns both go at once,
// which is what makes one run of -fix settle a package.
func (r *run) judge(p *packages.Package, candidates []*candidate) []Finding {
	r.held = map[string]bool{}
	for {
		reserved, claims, used := maps.Clone(r.reserved), maps.Clone(r.claims), maps.Clone(r.usedIgnores)
		findings, reported, fixed := r.judgeOnce(candidates)
		more := r.heldBy(p, reported, fixed)
		if len(more) == 0 {
			return findings
		}
		// The fixes of this round claimed names and marked ignores used, as
		// if they stood. The next round decides again from where this began.
		r.reserved, r.claims, r.usedIgnores = reserved, claims, used
		maps.Copy(r.held, more)
	}
}

// judgeOnce judges every candidate against what this round holds, and
// returns the findings with the keys of the candidates it reports and of
// those it would unexport.
func (r *run) judgeOnce(candidates []*candidate) (findings []Finding, reported, fixed map[string]bool) {
	reported, fixed = map[string]bool{}, map[string]bool{}
	for _, c := range candidates {
		if r.usedOutside(c) {
			continue
		}
		f := Finding{
			Pos:  r.mod.Fset.PositionFor(c.obj.Pos(), false),
			Rule: rule.Overexported,
			// A method or field is named alone, never with its type: the type
			// may be fixed in the same run, and a report the run keeps must
			// read the same afterwards.
			Name:    c.obj.Name(),
			Kind:    string(c.kind),
			Package: c.pkg.PkgPath,
		}
		if r.ev.extTest[c.key] {
			// Exporting from an in-package _test.go file for the external
			// tests is the export_test.go idiom, doing its job.
			if c.testFile {
				continue
			}
			f.TestOnly, f.Withheld = true, "external tests use it"
		} else {
			f.Withheld = r.withheld(c)
		}
		if r.silencedByIgnore(c) {
			continue
		}
		// The rename is asked last, once the report is known to stand. It
		// claims its new name for the rest of the run, and a silenced
		// declaration is never renamed, so it must claim nothing.
		if f.Withheld == "" {
			f.Fix, f.Withheld = r.renameEdits(c)
		}
		if f.Fix != nil {
			fixed[c.key] = true
		}
		reported[c.key] = true
		findings = append(findings, f)
	}
	return findings, reported, fixed
}

// heldBy returns every type of the package that a declaration staying
// exported carries, and that this round reports, with a fix or without: a
// carried type is used. The roots are the package's exported declarations,
// less those fixed; a fixed method or field is not followed either.
//
// Each root is walked on its own, so that a type does not hold itself. A
// type whose own fix is withheld stays exported, but that says nothing about
// whether anything uses it, and its report stays.
func (r *run) heldBy(p *packages.Package, reported, fixed map[string]bool) map[string]bool {
	fset := r.mod.Fset
	more := map[string]bool{}
	scope := p.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() || fixed[keyOf(fset, obj)] {
			continue
		}
		reach.ByName([]types.Type{obj.Type()}, func(o types.Object) bool { return !fixed[keyOf(fset, o)] }, func(tn *types.TypeName) {
			if key := keyOf(fset, tn); reported[key] && tn != obj {
				more[key] = true
			}
		})
	}
	return more
}

// usedOutside reports whether anything outside the candidate's package uses
// it. Each entry is one way to be used, and any of them silences the report.
func (r *run) usedOutside(c *candidate) bool {
	ev, key := r.ev, c.key
	uses := []func() bool{
		// Another package names it, writes it unkeyed, or links it by name.
		func() bool { return ev.outside[key] },
		// A static interface satisfaction needs it, and names no method.
		func() bool { return ev.satisfied[key] },
		// Another module reaches it through a value an importable package
		// hands out, or names it as an embedded field of one.
		func() bool { return ev.exposed[key] },
		// An API another package uses carries the type out, or an exported
		// declaration that stays exported does, and the type must stay
		// nameable where it is held.
		func() bool { return c.kind == kindType && (ev.carried[key] || r.held[key]) },
		// A struct conversion or an identical unnamed struct pairs the field
		// by name.
		func() bool { return c.kind == kindField && ev.paired[key] },
		// A struct tag says reflection reads the field, whether or not a value
		// reaches it here. go vet rejects a json or xml tag on an unexported
		// field, so the rename would not even pass vet.
		func() bool { return c.tagged },
		// A value of it escapes into an interface, where fmt, encoding/json
		// and reflect find methods and fields at run time, and %T prints a
		// type's name. A method escapes with its owner.
		func() bool { return ev.escaped[key] },
		func() bool { return c.kind == kindMethod && ev.escaped[keyOf(r.mod.Fset, c.owner)] },
		// A build-excluded file of another package writes pkg.Name.
		func() bool { return !c.kind.member() && c.qualifiedIn(r.mod.Excluded) },
	}
	return slices.ContainsFunc(uses, func(used func() bool) bool { return used() })
}

// withheld returns why the fix is withheld for a reason other than the
// rename itself, or "". Each entry is a use that may exist but cannot be
// proved, or a reference the rename cannot rewrite.
func (r *run) withheld(c *candidate) string {
	xs := r.mod.Excluded
	reasons := []struct {
		why     string
		applies func() bool
	}{
		{"a build-excluded file of another package may use it", func() bool { return c.maybeUsedIn(xs) }},
		{"a build-excluded file of its package names it", func() bool { return c.writtenInOwn(xs, c.obj.Name()) }},
		{"a generated file names it", func() bool { return r.ev.generated[c.key] }},
		{"an example function names it", func() bool { return r.ev.example[c.key] }},
		{"a string variable may be set by -ldflags -X, which names it", c.linkerSet},
	}
	for _, reason := range reasons {
		if reason.applies() {
			return reason.why
		}
	}
	return ""
}

// keyOf identifies a declaration across the variants of its package.
//
// A package with tests is type-checked more than once, and each variant has
// its own objects. They share the parsed files, so the declaring identifier's
// place is the one identity they agree on.
//
//declscope:package // every stage keys its evidence by it
func keyOf(fset *token.FileSet, obj types.Object) string {
	if obj == nil || !obj.Pos().IsValid() {
		return ""
	}
	p := fset.PositionFor(obj.Pos(), false)
	return fmt.Sprintf("%s:%d", p.Filename, p.Offset)
}

// origin maps an instantiated field or method back to the declared object:
// go/types records the instantiated one in Uses for a selection on a generic
// type. The analyzer's own origin does the same for the same reason.
//
//declscope:package // evidence.go and rename.go normalize every object with it
func origin(obj types.Object) types.Object {
	switch o := obj.(type) {
	case *types.Var:
		return o.Origin()
	case *types.Func:
		return o.Origin()
	}
	return obj
}

// embeddedTypeName returns the type name an embedded field is spelled with,
// and nil for any other object. A selection of the field (s.T) is spelled
// with the type's name, so renaming the type rewrites it too.
//
//declscope:package // evidence.go and rename.go follow embedded fields
func embeddedTypeName(obj types.Object) *types.TypeName {
	v, ok := obj.(*types.Var)
	if !ok || !v.Embedded() {
		return nil
	}
	t := v.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	switch t := t.(type) {
	case *types.Named:
		return t.Origin().Obj()
	case *types.Alias:
		return t.Obj()
	}
	return nil
}
