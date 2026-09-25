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
// wrong where a use is possible but not provable: a type that escapes into an
// interface, found at run time by fmt, encoding/json or reflect, or a name a
// build-excluded file writes. Each such doubt withholds the fix and leaves the
// report. The fix is made only where no use can exist outside the package.
//
// This file is the package's core: the model every stage reads, and the run
// that drives them. In a named namespace each type here would have to carry
// that namespace's name.
//
//declscope:core
package shrink

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
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
	mod, err := loadModule(dir)
	if err != nil {
		return Result{}, err
	}
	wanted, err := loadWanted(dir, patterns)
	if err != nil {
		return Result{}, err
	}
	ev, err := evidenceCollect(mod)
	if err != nil {
		return Result{}, err
	}
	r := &run{mod: mod, ev: ev, reserved: map[string]bool{}, facts: map[string]*renameFacts{}, usedIgnores: map[token.Pos]bool{}}
	var res Result
	for _, path := range mod.paths {
		if !wanted[path] {
			continue
		}
		pkgs := mod.byPath[path]
		if why := r.skip(pkgs); why != "" {
			// A package outside internal/ is never judged, and listing every
			// one would bury the packages a reader expected judged.
			if _, internal := loadInternalParent(path); internal {
				res.Skipped = append(res.Skipped, Skipped{Package: path, Reason: why})
			}
			continue
		}
		candidates, siblings := loadCandidates(pkgs)
		res.Findings = append(res.Findings, r.judge(candidates)...)
		res.Findings = append(res.Findings, r.unusedIgnores(pkgs, siblings)...)
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
//declscope:package // load.go and rename.go classify candidates by it
type kind string

//declscope:package // load.go classifies each candidate
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
//declscope:package // load.go and rename.go ask it
func (k kind) member() bool { return k == kindMethod || k == kindField }

// candidate is an exported declaration of a judged package.
//
//declscope:package // load.go and rename.go read what the judgment is about
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
}

// run is the state one invocation shares across the packages it judges.
//
//declscope:package // rename.go's renameEdits is a method of it
type run struct {
	mod *loadedModule
	ev  *evidence
	// reserved holds every new name a fix in this run has claimed. Two
	// exported names can lower to one (Foo and FOO both become foo), and
	// the fixes cannot see each other.
	reserved map[string]bool
	// facts caches what the rename guards ask of each package.
	facts map[string]*renameFacts
	// usedIgnores is every ignore naming overexported that silenced a report.
	//
	//declscope:private // only the core accounts for ignores
	usedIgnores map[token.Pos]bool
}

// skip says why a package is not judged at all, or returns "".
//
// Each reason is one way an importer could be unseen or a name could be
// read where the analysis never looks, so nothing is said rather than a
// guess.
func (r *run) skip(pkgs []*packages.Package) string {
	p := pkgs[0]
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
	_, ok := r.mod.rangeOf(p.PkgPath)
	if !ok {
		return "not inside an internal/ whose importers this run loads"
	}
	return ""
}

// judge reports every candidate of one package.
func (r *run) judge(candidates []*candidate) []Finding {
	var findings []Finding
	for _, c := range candidates {
		f, ok := r.judgeOne(c)
		if !ok {
			continue
		}
		if r.silenced(c) {
			continue
		}
		// The rename is asked last, once the report is known to stand. It
		// claims its new name for the rest of the run, and a silenced
		// declaration is never renamed, so it must claim nothing.
		if f.Withheld == "" {
			edits, why := r.renameEdits(c)
			if why != "" {
				f.Withheld = why
			} else {
				f.Fix = edits
			}
		}
		findings = append(findings, f)
	}
	return findings
}

// judgeOne reports one candidate, or returns false when something uses it.
// A finding with no Withheld reason is one the rename is still to decide.
func (r *run) judgeOne(c *candidate) (Finding, bool) {
	ev := r.ev
	if ev.outside[c.key] || ev.satisfied[c.key] {
		return Finding{}, false
	}
	// Another module can reach a method or field through a value, and an
	// embedded type's name as the name of the embedded field.
	if ev.exposed[c.key] {
		return Finding{}, false
	}
	// A type an API used from another package returns or takes is held
	// there, and must stay nameable where it is held.
	if c.kind == kindType && ev.carried[c.key] {
		return Finding{}, false
	}
	if c.owner != nil && ev.paired[c.key] {
		return Finding{}, false
	}
	xs := r.mod.excluded
	if !c.kind.member() && loadQualifiedInExcluded(xs, c) {
		return Finding{}, false
	}
	f := Finding{
		Pos:     r.mod.fset.PositionFor(c.obj.Pos(), false),
		Rule:    rule.Overexported,
		Name:    candidateName(c),
		Kind:    string(c.kind),
		Package: c.pkg.PkgPath,
	}
	if ev.extTest[c.key] {
		// Exporting from an in-package _test.go file for the external tests
		// is the export_test.go idiom, and doing its job.
		if c.testFile {
			return Finding{}, false
		}
		f.TestOnly = true
		f.Withheld = "external tests use it"
		return f, true
	}
	switch {
	case r.escaped(c):
		f.Withheld = "its type escapes into an interface, where reflection may find it"
	case loadNamedInOtherExcluded(xs, c):
		f.Withheld = "a build-excluded file of another package may use it"
	case loadNamedInOwnExcluded(xs, c, c.obj.Name()):
		f.Withheld = "a build-excluded file of its package names it"
	case ev.generated[c.key]:
		f.Withheld = "a generated file names it"
	case ev.example[c.key]:
		f.Withheld = "an example function names it"
	case candidateLinkerSet(c):
		f.Withheld = "a string variable may be set by -ldflags -X, which names it"
	}
	return f, true
}

// escaped reports whether a value that reflection may inspect reaches the
// candidate: the type itself for a type, the owner for a method, and the
// field itself for a field, which every type sharing the struct holds.
func (r *run) escaped(c *candidate) bool {
	switch c.kind {
	case kindType:
		return r.ev.escaped[c.key]
	case kindMethod:
		return r.ev.escaped[keyOf(r.mod.fset, c.owner)]
	case kindField:
		return r.ev.escaped[c.key]
	}
	return false
}

// silenced reports whether an ignore naming overexported covers the
// candidate, and records every one that does. A bare ignore does not reach a
// module-wide rule: see rule.IsModuleWide.
func (r *run) silenced(c *candidate) bool {
	hit := false
	for _, ig := range c.ignores {
		if slices.Contains(ig.Rules, rule.Overexported) {
			r.usedIgnores[ig.Pos] = true
			hit = true
		}
	}
	return hit
}

// unusedIgnores reports every ignore naming nothing but overexported, in the
// files of a judged package, that silenced nothing. One naming another rule
// as well is judged by neither side: the analyzer cannot see this rule, and
// this run cannot see the analyzer's.
//
// The report is the unused rule's, answered as the analyzer answers its own:
// by an ignore beside it naming unused, or by a file-level ignore covering
// unused, bare or named. The analyzer in turn never calls such an answer
// unused, since it cannot see the report it answers.
func (r *run) unusedIgnores(pkgs []*packages.Package, siblings map[token.Pos][]directive.Ignore) []Finding {
	var findings []Finding
	seen := map[token.Pos]bool{}
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			// A generated file declares no candidate, so an ignore there
			// could never silence one. The analyzer does not read it either.
			if ast.IsGenerated(file) {
				continue
			}
			fileIgnores := directive.ParseFile(file).Ignores
			for _, g := range file.Comments {
				group := directive.ParseDecl(g).Ignores
				for _, ig := range group {
					if seen[ig.Pos] || r.usedIgnores[ig.Pos] || !unusedJudged(ig) {
						continue
					}
					seen[ig.Pos] = true
					// The siblings are those of the binding the ignore was read
					// in, doc and trailing comment together. An ignore bound to
					// no candidate has only its own comment group.
					beside, ok := siblings[ig.Pos]
					if !ok {
						beside = group
					}
					if unusedAnswered(ig, beside, fileIgnores) {
						continue
					}
					findings = append(findings, Finding{
						Pos:     r.mod.fset.PositionFor(ig.Pos, false),
						Rule:    rule.Unused,
						Package: p.PkgPath,
					})
				}
			}
		}
	}
	return findings
}

// unusedJudged reports whether this run judges ig: it names module-wide
// rules and nothing else.
func unusedJudged(ig directive.Ignore) bool {
	return len(ig.Rules) > 0 && !slices.ContainsFunc(ig.Rules, func(x rule.Rule) bool { return !rule.IsModuleWide(x) })
}

// unusedAnswered reports whether another ignore answers ig's unused report:
// one beside it naming unused, or a file-level one covering it. No ignore
// answers its own report.
func unusedAnswered(ig directive.Ignore, beside, fileIgnores []directive.Ignore) bool {
	for _, other := range beside {
		if other.Pos != ig.Pos && slices.Contains(other.Rules, rule.Unused) {
			return true
		}
	}
	for _, other := range fileIgnores {
		if other.Pos != ig.Pos && other.Covers(rule.Unused) {
			return true
		}
	}
	return false
}

// candidateLinkerSet reports whether the candidate is a package-level string
// variable, which `go build -ldflags "-X path.Name=value"` sets by name. A
// build file passes the flag, where go/types never looks, and the linker
// ignores a -X whose name no longer exists, so the rename would break the
// release silently.
func candidateLinkerSet(c *candidate) bool {
	v, ok := c.obj.(*types.Var)
	if !ok || c.kind != kindVar {
		return false
	}
	b, ok := v.Type().Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// candidateName is the name the message uses. A method or field is named
// alone, never with its type: the type may be fixed in the same run, and a
// report the run keeps must read the same afterwards.
func candidateName(c *candidate) string { return c.obj.Name() }

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
//declscope:package // evidence.go records such selections under the type
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
