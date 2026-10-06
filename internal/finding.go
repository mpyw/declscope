package internal

import (
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// finding is a diagnostic that a target would produce, held back until
// its ignore directives and the baseline have been consulted.
//
// This file decides the findings; report.go reports them and survey.go counts
// them, both from findingsOf, so the two cannot disagree about what is a
// violation.
//
//declscope:package // report.go reports it and survey.go counts it
type finding struct {
	rule    rule.Rule
	decl    string
	pos     token.Pos
	msg     string
	related []analysis.RelatedInformation
	fixes   []analysis.SuggestedFix
	// settledBy is the type whose own finding stands in for this one, when
	// that finding is not absorbed by the baseline. See settled.
	//
	//declscope:private // only settled reads it
	settledBy *target
}

// settled reports whether f is strict's finding on a member whose
// type is reported in its place: the type's fix narrows the member too, so a
// second report would say the same thing.
//
// The type stands in only while its finding is not absorbed by the baseline.
// A baselined type offers no fix, and a member added to it since must be
// reported, since a baseline records what a codebase already has and still
// reports what is new. An ignore needs no test: every ignore that silences the
// type's finding is on the member's chain too, and silences it there.
//
// Regeneration does not ask. It records the members with their type, because
// the baseline it writes is what absorbs the type.
//
//declscope:package // report.go and survey.go both drop what a type settles
func (f finding) settled(pass *analysis.Pass, opts Options) bool {
	o := f.settledBy
	if o == nil {
		return false
	}
	return !opts.Baseline.Has(finding{rule: rule.Surplus, decl: o.name()}.key(pass, o))
}

// key identifies the finding for the baseline, independently of position.
//
//declscope:package // report.go and survey.go both look the finding up in the baseline
func (f finding) key(pass *analysis.Pass, t *target) baseline.Key {
	return baseline.Key{
		Package:   pass.Pkg.Path(),
		Rule:      f.rule,
		Namespace: t.file.spelledNamespace(),
		Decl:      f.decl,
	}
}

// findingsOf returns every finding of one target, before any ignore or the
// baseline is consulted.
//
//declscope:package // report.go reports them, and survey.go counts them
func (c *collection) findingsOf(pass *analysis.Pass, opts Options, t *target) []finding {
	var out []finding
	// An exported declaration resolves to package scope unless a directive
	// narrows it, so the one test below covers both: what is reachable from
	// outside carries no boundary, and what an author narrowed does.
	if opts.Boundary.Reports() && t.scope == scope.Private {
		if f, ok := c.boundaryFinding(pass, opts, t); ok {
			out = append(out, f)
		}
	}
	if f, ok := c.qualifyFinding(pass, opts, t); ok {
		out = append(out, f)
	}
	// The judgment and the wording both live in surplus.go; only the finding
	// is assembled here, so that the ignore chain and the baseline treat the
	// rule like any other.
	if pos, msg, ok := c.checkSurplus(pass, opts, t); ok {
		out = append(out, finding{rule: rule.Surplus, decl: t.name(), pos: pos, msg: msg})
	}
	// strict's finding on one declaration shares the rule's name, so the same
	// ignore and the same baseline key answer for it. surplus.go words it and
	// decides the fix.
	if msg, fix, settledBy, ok := c.checkSurplusDeclaration(pass, opts, t); ok {
		f := finding{rule: rule.Surplus, decl: t.name(), pos: t.ident.Pos(), msg: msg, settledBy: settledBy}
		if fix != nil {
			f.fixes = append(f.fixes, *fix)
		}
		out = append(out, f)
	}
	return out
}

// boundaryFinding reports a declaration that is private to its namespace but is
// referenced from outside it.
func (c *collection) boundaryFinding(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	var offenders []ref
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.file.key() {
			offenders = append(offenders, r)
		}
	}
	if len(offenders) == 0 {
		return finding{}, false
	}

	f := finding{rule: rule.Boundary, decl: t.name(), pos: t.ident.Pos()}
	// The message names the level that decided, not the level a reader might
	// assume: a field takes its type's directive and any declaration takes its
	// file's, and naming the declaration's own would point at a comment that is
	// not there.
	switch t.boundAt {
	case boundAtDecl:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), fileInFinding(offenders[0].file))
	case boundAtContainer:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s on %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), t.owner, fileInFinding(offenders[0].file))
	case boundAtFile:
		f.msg = fmt.Sprintf("%s %s is declared %s by the file's %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), fileInFinding(offenders[0].file))
	default:
		f.msg = fmt.Sprintf("%s %s is private to %s, but is used from %s",
			t.kind, t.name(), fileInFinding(t.file), fileInFinding(offenders[0].file))
	}
	for _, r := range offenders {
		f.related = append(f.related, analysis.RelatedInformation{
			Pos:     r.node.Pos(),
			End:     r.node.End(),
			Message: fmt.Sprintf("used here, in %s", fileInFinding(r.file)),
		})
	}

	// When the author stated the scope ON THE DECLARATION, the conflict is
	// between two explicit decisions and only they can resolve it: surplus
	// would have -fix silently overwrite the directive they wrote.
	//
	// A scope inherited from the containing type or from the file is a default,
	// and a directive inserted on the declaration states an exception to it
	// rather than overwriting a decision — but inserting one would also leave
	// the outer directive binding one declaration fewer, which can make it
	// unused. Offering the fix there would produce a diagnostic that did not
	// exist before, so it is withheld at every level that supplied the scope.
	if t.boundAt != boundAtDefault {
		return f, true
	}
	fix := c.directiveInsertionFix(pass, t, scope.PackageInternal)
	// The directive reaches the type's members too, so under strict a member
	// no other namespace reads would come out of this fix package-scoped, and
	// surplus would report what -fix had just written. The fix narrows those
	// members in the same edit, which is the state the two rules agree on;
	// surplus.go decides which members they are.
	if t.kind == kindType {
		var names []string
		for _, m := range c.surplusNarrowedByTypeFix(pass, opts, t) {
			fix.TextEdits = append(fix.TextEdits, c.directiveInsertion(pass, m, scope.Private, m.doc))
			names = append(names, m.name())
		}
		if len(names) > 0 {
			fix.Message += ", and " + scope.Private.Directive() + " to " + strings.Join(names, ", ")
		}
	}
	f.fixes = append(f.fixes, fix)
	return f, true
}

// qualifyFinding requires a package-level declaration to carry its namespace
// somewhere in its name, and offers a prefix when it does not.
//
// The namespace grants nothing — reach is stated with a directive — so this is
// purely an ownership mark, making the owning unit legible at every use site
// and in every stack trace and grep result. It need not lead the name: Go puts
// a type's category last (statementReducer) and spells a constructor NewTracer,
// and demanding a prefix there doubled the word the name already carried.
// namespace.Contains is the test.
//
// Whether it applies at all depends on rules.naming.qualify, which defaults
// to never: the convention is opt-in. See rule.QualifyMode and DefaultOptions.
func (c *collection) qualifyFinding(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !c.qualifyFindingApplies(pass, opts, t) {
		return finding{}, false
	}
	name := t.obj.Name()
	if namespace.Contains(name, t.file.ns) {
		return finding{}, false
	}
	// A configured vocabulary word carries the namespace the way its own
	// spelling would, under the same test: word boundary on the left, free
	// right edge. It widens what counts as carrying, never what is asked.
	if slices.ContainsFunc(opts.Vocabulary[t.file.ns], func(word string) bool { return namespace.Contains(name, word) }) {
		return finding{}, false
	}
	f := finding{
		rule: rule.Qualify,
		decl: name,
		pos:  t.ident.Pos(),
		// The rename is one answer, not the requirement. Naming only the
		// prefixed form would restate the prefix rule that containment
		// replaced, and push the author away from normalizeUserEmail and
		// userEmailFrom, which settle the rule just as well.
		msg: fmt.Sprintf("%s %s does not carry %s anywhere in its name; rename it to %s, or to another name that carries %q",
			t.kind, name, namespaceInFinding(t.file.ns, t.file.path),
			namespace.Qualify(name, t.file.ns), t.file.ns),
	}
	// An inflected namespace is not carried by its stem (see
	// namespace.LooksInflected), and the rename would stutter: tracingTraceValue.
	// The vocabulary is the answer, so the message names the key to add.
	if namespace.LooksInflected(t.file.ns) {
		f.msg += fmt.Sprintf("; if the name spells %q in another form, list that form under rules.naming.vocabulary.%s",
			t.file.ns, t.file.ns)
	}
	if fix, ok := c.renameFix(pass, t, namespace.Qualify(name, t.file.ns),
		"prefix it with its namespace"); ok {
		f.fixes = append(f.fixes, fix)
	}
	return f, true
}

// qualifyFindingApplies reports whether the naming rule asks anything of a
// declaration at all. It is the whole gate, in one place, because it is also
// the denominator every saturation divides by: a survey that counted one of
// these conjuncts would move a number without a diagnostic moving with it.
//
// A namespace is always an identity, but not always a prefix: 2fa.go bounds
// its declarations like any other file, yet no identifier can start with a
// digit. Containment alone could be satisfied there, by spelling the namespace
// later in the name, but the fix could not be, so the rule stays out rather
// than report what it cannot remedy.
//
// main is a name the toolchain requires, so nothing can be asked of it.
//
//declscope:package // the survey divides by it, and must divide by this one
func (c *collection) qualifyFindingApplies(pass *analysis.Pass, opts Options, t *target) bool {
	if !opts.Qualify.Applies(c.namespaces) || !t.findingReachesName(opts) {
		return false
	}
	if !namespace.CanPrefix(t.file.ns) {
		return false
	}
	return t.kind != kindFunc || t.obj.Name() != "main" || pass.Pkg.Name() != "main"
}

// namespaceInFinding names a file's namespace, or the file itself when its stem
// yields none (★.go). Every file has a path, taken from the file set.
func namespaceInFinding(ns, path string) string {
	if ns != "" {
		return fmt.Sprintf("namespace %q", ns)
	}
	return fmt.Sprintf("file %s", filepath.Base(path))
}

func fileInFinding(f *fileInfo) string {
	// The core namespace has no name, and the "file X.go" fallback in
	// namespaceInFinding was written for a file with no stem at all. Letting the
	// core fall into it would say "private to file client.go" about a
	// declaration every other core file may use — telling the reader something
	// the analyzer does not believe.
	if f.core {
		return "the core namespace"
	}
	return namespaceInFinding(f.ns, f.path)
}

// findingReachesName reports whether the naming rule reaches a declaration at all.
//
// It reaches an exported declaration when rules.naming.exported says so —
// inside the package an exported name is read as bare as any other, which is
// the reading the namespace mark exists for — and never reaches the core
// namespace, whose prefix is empty and so has nothing to require.
//
// A member never carries a namespace: it is written inside its type and read
// through it. A method with a receiver is read through the receiver too, which
// names the unit its type belongs to. That answers the ownership question only
// while the two are the same unit. A method filed away from its type points the
// reader at a namespace that does not hold it, so the rule reaches it and asks
// for the namespace it is actually written in.
func (t *target) findingReachesName(opts Options) bool {
	if t.file.core || t.findingSparesToolchainName() {
		return false
	}
	switch {
	case t.contained:
		return false
	case t.kind == kindMethod:
		if !t.foreignMethod() {
			return false
		}
	}
	return !isExported(t.obj.Name()) || opts.NameExported
}

// findingSparesToolchainName reports whether the toolchain finds this declaration by its
// name, so that no rename can be asked of it. `go test` collects a test by
// name, and renaming TestLoad to userTestLoad leaves a function nothing runs.
//
// The match is looser than the toolchain's own, which also reads the signature
// and requires that TestXxx's Xxx not begin with a lowercase letter. Erring
// toward exempting is the safe direction here: exempting one name too many
// costs a rename nobody asked for, and exempting one too few is advice that
// breaks the build.
func (t *target) findingSparesToolchainName() bool {
	if t.kind != kindFunc || !strings.HasSuffix(t.file.path, "_test.go") {
		return false
	}
	name := t.obj.Name()
	return slices.ContainsFunc([]string{"Test", "Benchmark", "Fuzz", "Example"}, func(prefix string) bool {
		return strings.HasPrefix(name, prefix)
	})
}
