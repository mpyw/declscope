package internal

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/measure"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// reportedFinding is a diagnostic that a target would produce, held back until
// its ignore directives and the baseline have been consulted.
type reportedFinding struct {
	rule    rule.Rule
	decl    string
	pos     token.Pos
	msg     string
	related []analysis.RelatedInformation
	fixes   []analysis.SuggestedFix
	// settledBy is the type whose own finding stands in for this one, when
	// that finding is not absorbed by the baseline. See settledInReport.
	settledBy *target
}

// pendingReport is one target's surviving findings, held until every target
// has been judged. A fix is decided against the pre-fix source, so what one
// fix is about to write can only be known once they all exist.
type pendingReport struct {
	t        *target
	findings []reportedFinding
}

// widensReport reports whether this finding's fix widens the declaration to
// package scope, and with it every member the declaration contains. Only a
// type contains members, and only an inserted directive widens: a rename
// changes no reach at all.
func (f reportedFinding) widensReport(t *target) bool {
	return f.rule == rule.Boundary && len(f.fixes) > 0 && t.kind == kindType && !t.contained
}

// subsumedByReport reports whether the fix a member's finding carries is
// already going to be written by the fix on its type, in the same run.
//
// Every fix is decided against the pre-fix source and none of them can see the
// others, so two can converge — the shape spec/rename_siblings.fsl already
// models for renames. A directive inserted on a type reaches the type's
// members, which leaves a directive inserted on a member in the same run
// binding nothing: the directive rule then reports what -fix just wrote, and a
// second -fix does not clear it, because that rule carries no fix. The wider
// insertion wins and the narrower one is dropped.
//
// Only the fix is dropped, never the diagnostic. The member really is reached
// from another namespace, and it says so whether or not the author applies the
// fix offered on its type.
//
// The member's own directive can only be reached here when the configured
// default supplied its scope — anything stated on the member, on its type or
// in its file withholds the fix already — and in that case nothing stated the
// type's scope either, so the type carries the insertion whenever it crosses
// at all.
func subsumedByReport(t *target, f reportedFinding, widened map[types.Object]bool) bool {
	return f.rule == rule.Boundary && t.contained && t.ownerObj != nil && widened[t.ownerObj]
}

// report renders every diagnostic of the pass.
//
//declscope:package // the analyzer's reporting entry, driven from analyzer.go
func (c *collection) report(pass *analysis.Pass, opts Options) {
	// The order decides which of two fixes claiming the same new name gets
	// it, so it must be the same in every run. token.Pos alone is not:
	// go/packages parses files concurrently, so the order in which they
	// enter the FileSet, and with it the relative order of positions in
	// different files, differs from one run to the next.
	slices.SortStableFunc(c.targets, func(a, b *target) int {
		return comparePos(pass.Fset, a.ident.Pos(), b.ident.Pos())
	})

	// Every surviving finding is collected before any of them is reported,
	// because one fix can subsume another and no fix can see the edits of the
	// others. widensReport marks what one is about to widen; subsumedByReport
	// spells out why.
	pending := make([]pendingReport, 0, len(c.targets))
	widened := make(map[types.Object]bool)
	for _, t := range c.targets {
		p := pendingReport{t: t}
		for _, f := range c.findingsForReport(pass, opts, t) {
			if f.settledInReport(pass, opts) {
				continue
			}
			// Ignores are consulted before the baseline: a suppression the
			// baseline would also have absorbed still counts as the directive
			// doing its job.
			if c.silencedByIgnore(t, f.rule) {
				continue
			}
			if opts.Baseline.Has(f.key(pass, t)) {
				continue
			}
			if f.widensReport(t) {
				widened[t.obj] = true
			}
			p.findings = append(p.findings, f)
		}
		if len(p.findings) > 0 {
			pending = append(pending, p)
		}
	}

	for _, p := range pending {
		for _, f := range p.findings {
			if subsumedByReport(p.t, f, widened) {
				f.fixes = nil
			}
			pass.Report(analysis.Diagnostic{
				Pos:            f.pos,
				Category:       string(f.rule),
				Message:        f.msg,
				Related:        f.related,
				SuggestedFixes: f.fixes,
			})
		}
	}

	// Unused directives are reported only once every finding has been seen,
	// since a type's directive may be used up by one of its members, which is
	// reached later in the loop above.
	c.reportUnusedScopeSites(pass)

	// Directive hygiene carries a rule like every other check, so that
	// //declscope:ignore directive can silence one. A report with no rule is
	// one nothing could answer.
	//
	// It takes no baseline entry, and wants none: a baseline exists so that
	// turning declscope on does not report boundaries a codebase never
	// enforced, which is history nobody can edit away. A directive the author
	// wrote is not history — removing it removes the report.
	//
	// Silencing is settled before the unused-ignore report, not after: an
	// ignore written for a directive problem silences something attached to no
	// declaration, so the per-target accounting never sees it work, and
	// reporting it unused first would tell the author to delete the very
	// comment doing the job.
	slices.SortStableFunc(c.problems, func(a, b directive.Problem) int {
		return comparePos(pass.Fset, a.Pos, b.Pos)
	})
	surviving := c.problems[:0]
	for _, p := range c.problems {
		if c.ignoreSilencesFile(c.fileAt(pass, p.Pos), rule.Directive) {
			continue
		}
		surviving = append(surviving, p)
	}
	c.problems = surviving

	// Only now, with every ignore that silencedByIgnore something marked used. An
	// ignore report is itself a directive problem, so it is appended rather
	// than reported directly, and joins the same ordering — and the same
	// silencing, which is why the filter runs again over the tail. Running it
	// once would leave the one report nothing could answer.
	c.reportUnusedIgnores(pass)
	kept := c.problems[:0]
	for _, p := range c.problems {
		if c.ignoreSilencesFile(c.fileAt(pass, p.Pos), rule.Directive) {
			continue
		}
		kept = append(kept, p)
	}
	c.problems = kept

	for _, p := range c.problems {
		pass.Report(analysis.Diagnostic{
			Pos:      p.Pos,
			Category: string(rule.Directive),
			Message:  p.Msg,
		})
	}
	if w := c.filterWarning; w != nil {
		pass.Report(analysis.Diagnostic{
			Pos:      w.Pos,
			Category: string(rule.Filter),
			Message:  w.Msg,
		})
	}
}

// namespaceForReport spells the namespace for the baseline, which is the one place it has
// to be written down rather than compared.
//
// A namespace derived from a file name is normalized to alphanumerics, and one
// written with //declscope:namespace must be an unexported identifier, so a
// parenthesis can appear in neither. That leaves "(core)" free for the core,
// whose prefix is empty and whose files all share it, and free for the file
// with no stem at all — which has no namespace either, and must not share a
// key with every other such file.
//
//declscope:package // the survey spells every namespace with it too
func (f *fileInfo) namespaceForReport() string {
	if f.core {
		return "(core)"
	}
	if f.ns != "" {
		return f.ns
	}
	return "(file " + filepath.Base(f.path) + ")"
}

// settledInReport reports whether f is strict's finding on a member whose
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
func (f reportedFinding) settledInReport(pass *analysis.Pass, opts Options) bool {
	o := f.settledBy
	if o == nil {
		return false
	}
	return !opts.Baseline.Has(reportedFinding{rule: rule.Surplus, decl: o.name()}.key(pass, o))
}

// key identifies the finding for the baseline, independently of position.
func (f reportedFinding) key(pass *analysis.Pass, t *target) baseline.Key {
	return baseline.Key{
		Package:   pass.Pkg.Path(),
		Rule:      f.rule,
		Namespace: t.file.namespaceForReport(),
		Decl:      f.decl,
	}
}

// keysForReport returns every violation the pass would report, ignoring the baseline.
// It is what regenerating a baseline records.
//
//declscope:package // the baseline regeneration entry, driven from analyzer.go
func (c *collection) keysForReport(pass *analysis.Pass, opts Options) []baseline.Key {
	var out []baseline.Key
	for _, t := range c.targets {
		for _, f := range c.findingsForReport(pass, opts, t) {
			if c.silencedByIgnore(t, f.rule) {
				continue
			}
			out = append(out, f.key(pass, t))
		}
	}
	return out
}

// surveyedFindingsForReport returns one target's findings together with what
// became of each: silenced by a directive, absorbed by the baseline, or
// reported.
//
// It is the only entry the survey uses, so that the survey never asks "is this
// a violation" a second way — a second answer would drift from this one, and
// the difference would read as a bug in one of them rather than as two
// different questions. Routing through here is also what keeps the order of
// the two suppressions the same: an ignore is consulted before the baseline,
// so a suppression the baseline would also have absorbed still counts as the
// directive doing its job.
//
//declscope:package // the survey's entry, driven from survey.go
func (c *collection) surveyedFindingsForReport(pass *analysis.Pass, opts Options, t *target) []measure.Finding {
	findings := c.findingsForReport(pass, opts, t)
	out := make([]measure.Finding, 0, len(findings))
	for _, f := range findings {
		if f.settledInReport(pass, opts) {
			continue
		}
		state := measure.FindingReported
		switch {
		case c.silencedByIgnore(t, f.rule):
			state = measure.FindingIgnored
		case opts.Baseline.Has(f.key(pass, t)):
			state = measure.FindingBaselined
		}
		out = append(out, measure.Finding{
			Rule:        f.rule,
			Declaration: f.decl,
			State:       state,
			// The fix the analyzer decided on, not a rerun of the decision:
			// renameFix reserves the name it claims, so asking again would
			// answer about a package that already contains this fix.
			Fixable: len(f.fixes) > 0,
		})
	}
	return out
}

// surveyedProblemsForReport tallies the two rules that carry no baseline key:
// the directive rule, whose findings are settled in a second pass once every
// other finding has been seen, and the filter rule, which reports at most once
// per package.
//
// It runs the same two stages report runs, in the same order, for the same
// reason: an ignore written for a directive problem silences something
// attached to no declaration, so it has to be consulted before the unused
// ignores are judged, or the author is told to delete the comment doing the
// job.
//
//declscope:package // the survey's second entry, driven from survey.go
func (c *collection) surveyedProblemsForReport(pass *analysis.Pass) (measure.Count, measure.Count) {
	count := measure.Count{Asked: true}

	c.reportUnusedScopeSites(pass)
	count.Found = len(c.problems)
	c.problems = c.silencedProblemsForReport(pass)

	kept := len(c.problems)
	c.reportUnusedIgnores(pass)
	count.Found += len(c.problems) - kept
	c.problems = c.silencedProblemsForReport(pass)

	count.Reported = len(c.problems)
	count.Ignored = count.Found - count.Reported

	filtered := measure.Count{Asked: true}
	if c.filterWarning != nil {
		filtered.Found, filtered.Reported = 1, 1
	}
	return count, filtered
}

// silencedProblemsForReport drops the problems a file-level ignore stands
// down, marking the directive that did it used.
func (c *collection) silencedProblemsForReport(pass *analysis.Pass) []directive.Problem {
	kept := c.problems[:0]
	for _, p := range c.problems {
		if c.ignoreSilencesFile(c.fileAt(pass, p.Pos), rule.Directive) {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

func (c *collection) findingsForReport(pass *analysis.Pass, opts Options, t *target) []reportedFinding {
	var out []reportedFinding
	// An exported declaration resolves to package scope unless a directive
	// narrows it, so the one test below covers both: what is reachable from
	// outside carries no boundary, and what an author narrowed does.
	if opts.Boundary.Reports() && t.scope == scope.Private {
		if f, ok := c.boundaryFindingForReport(pass, opts, t); ok {
			out = append(out, f)
		}
	}
	if f, ok := c.qualifyFindingForReport(pass, opts, t); ok {
		out = append(out, f)
	}
	// The judgment and the wording both live in surplus.go; only the finding
	// is assembled here, so that the ignore chain and the baseline treat the
	// rule like any other.
	if pos, msg, ok := c.checkSurplus(pass, opts, t); ok {
		out = append(out, reportedFinding{rule: rule.Surplus, decl: t.name(), pos: pos, msg: msg})
	}
	// strict's finding on one declaration shares the rule's name, so the same
	// ignore and the same baseline key answer for it. surplus.go words it and
	// decides the fix.
	if msg, fix, settledBy, ok := c.checkSurplusDeclaration(pass, opts, t); ok {
		f := reportedFinding{rule: rule.Surplus, decl: t.name(), pos: t.ident.Pos(), msg: msg, settledBy: settledBy}
		if fix != nil {
			f.fixes = append(f.fixes, *fix)
		}
		out = append(out, f)
	}
	return out
}

// boundaryFindingForReport reports a declaration that is private to its namespace but is
// referenced from outside it.
func (c *collection) boundaryFindingForReport(pass *analysis.Pass, opts Options, t *target) (reportedFinding, bool) {
	var offenders []ref
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.file.key() {
			offenders = append(offenders, r)
		}
	}
	if len(offenders) == 0 {
		return reportedFinding{}, false
	}

	f := reportedFinding{rule: rule.Boundary, decl: t.name(), pos: t.ident.Pos()}
	// The message names the level that decided, not the level a reader might
	// assume: a field takes its type's directive and any declaration takes its
	// file's, and naming the declaration's own would point at a comment that is
	// not there.
	switch t.boundAt {
	case scopesiteLevelDecl:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), fileInReport(offenders[0].file))
	case scopesiteLevelContainer:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s on %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), t.owner, fileInReport(offenders[0].file))
	case scopesiteLevelFile:
		f.msg = fmt.Sprintf("%s %s is declared %s by the file's %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), fileInReport(offenders[0].file))
	default:
		f.msg = fmt.Sprintf("%s %s is private to %s, but is used from %s",
			t.kind, t.name(), fileInReport(t.file), fileInReport(offenders[0].file))
	}
	for _, r := range offenders {
		f.related = append(f.related, analysis.RelatedInformation{
			Pos:     r.node.Pos(),
			End:     r.node.End(),
			Message: fmt.Sprintf("used here, in %s", fileInReport(r.file)),
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
	if t.boundAt != scopesiteLevelDefault {
		return f, true
	}
	fix := directiveFixInReport(pass, t, scope.PackageInternal)
	// The directive reaches the type's members too, so under strict a member
	// no other namespace reads would come out of this fix package-scoped, and
	// surplus would report what -fix had just written. The fix narrows those
	// members in the same edit, which is the state the two rules agree on;
	// surplus.go decides which members they are.
	if t.kind == kindType {
		var names []string
		for _, m := range c.surplusNarrowedByTypeFix(pass, opts, t) {
			fix.TextEdits = append(fix.TextEdits, directiveEditInReport(pass, m, scope.Private, m.doc))
			names = append(names, m.name())
		}
		if len(names) > 0 {
			fix.Message += ", and " + scope.Private.Directive() + " to " + strings.Join(names, ", ")
		}
	}
	f.fixes = append(f.fixes, fix)
	return f, true
}

// qualifyFindingForReport requires a package-level declaration to carry its namespace
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
// to never: the convention is opt-in. See QualifyMode and DefaultOptions.
func (c *collection) qualifyFindingForReport(pass *analysis.Pass, opts Options, t *target) (reportedFinding, bool) {
	if !c.qualifyExaminesForReport(pass, opts, t) {
		return reportedFinding{}, false
	}
	name := t.obj.Name()
	if namespace.Contains(name, t.file.ns) {
		return reportedFinding{}, false
	}
	// A configured vocabulary word carries the namespace the way its own
	// spelling would, under the same test: word boundary on the left, free
	// right edge. It widens what counts as carrying, never what is asked.
	for _, word := range opts.Vocabulary[t.file.ns] {
		if namespace.Contains(name, word) {
			return reportedFinding{}, false
		}
	}
	f := reportedFinding{
		rule: rule.Qualify,
		decl: name,
		pos:  t.ident.Pos(),
		// The rename is one answer, not the requirement. Naming only the
		// prefixed form would restate the prefix rule that containment
		// replaced, and push the author away from normalizeUserEmail and
		// userEmailFrom, which settle the rule just as well.
		msg: fmt.Sprintf("%s %s does not carry %s anywhere in its name; rename it to %s, or to another name that carries %q",
			t.kind, name, namespaceInReport(t.file.ns, t.file.path),
			namespace.Qualify(name, t.file.ns), t.file.ns),
	}
	if fix, ok := c.renameFix(pass, t, namespace.Qualify(name, t.file.ns),
		"prefix it with its namespace"); ok {
		f.fixes = append(f.fixes, fix)
	}
	return f, true
}

// qualifyExaminesForReport reports whether the naming rule asks anything of a
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
func (c *collection) qualifyExaminesForReport(pass *analysis.Pass, opts Options, t *target) bool {
	if !opts.Qualify.Applies(c.namespaces) || !t.reportsName(opts) {
		return false
	}
	if !namespace.CanPrefix(t.file.ns) {
		return false
	}
	return t.kind != kindFunc || t.obj.Name() != "main" || pass.Pkg.Name() != "main"
}

// directiveFixInReport inserts an explicit scope directive above the declaration.
func directiveFixInReport(pass *analysis.Pass, t *target, s scope.Scope) analysis.SuggestedFix {
	return analysis.SuggestedFix{
		Message:   fmt.Sprintf("add %s to %s", s.Directive(), t.name()),
		TextEdits: []analysis.TextEdit{directiveEditInReport(pass, t, s, nil)},
	}
}

// directiveEditInReport is the insertion itself.
//
// A directive only binds to a declaration when it sits on its own line above
// it, so a declaration that shares a line with something else — a field of a
// single-line struct, for instance — first has to be broken onto a line of its
// own. The formatter applied to the fixed file restores the indentation.
//
// doc is the comment the directive lands under, when the caller wants the
// house layout: a doc comment, a bare // line, then the directive. go/doc
// strips a directive from the rendered text either way, but the blank comment
// line is what keeps the two readable as separate things in the source. A doc
// comment that already ends in a directive or in a bare // needs no separator.
//
//declscope:package // surplus.go narrows a declaration with the same insertion
func directiveEditInReport(pass *analysis.Pass, t *target, s scope.Scope, doc *ast.CommentGroup) analysis.TextEdit {
	var text string
	if atLineStartForReport(pass, t.anchor) {
		indent := strings.Repeat("\t", max(pass.Fset.Position(t.anchor).Column-1, 0))
		text = s.Directive() + "\n" + indent
		if doc != nil && len(doc.List) > 0 && !endsInDirectiveForReport(doc) {
			text = "//\n" + indent + text
		}
	} else {
		text = "\n" + s.Directive() + "\n"
	}
	return analysis.TextEdit{Pos: t.anchor, End: t.anchor, NewText: []byte(text)}
}

// directiveLineForReport is the shape go/ast recognizes as a directive: no
// space after the slashes, a lowercase word, and a colon. The three spellings
// without a colon are the ones cgo and the linker read.
var directiveLineForReport = regexp.MustCompile(`^//(line |extern |export |[a-z0-9]+:[a-z0-9])`)

func endsInDirectiveForReport(doc *ast.CommentGroup) bool {
	last := doc.List[len(doc.List)-1].Text
	return last == "//" || directiveLineForReport.MatchString(last)
}

// atLineStartForReport reports whether pos is preceded on its line by nothing but
// whitespace. It fails safe: an unreadable file is treated as not starting a
// line, which yields an extra line break rather than a misplaced directive.
func atLineStartForReport(pass *analysis.Pass, pos token.Pos) bool {
	position := pass.Fset.Position(pos)
	if position.Column <= 1 {
		return true
	}
	if pass.ReadFile == nil {
		return false
	}
	content, err := pass.ReadFile(position.Filename)
	if err != nil || position.Offset > len(content) {
		return false
	}
	prefix := content[position.Offset-position.Column+1 : position.Offset]
	return strings.TrimLeft(string(prefix), " \t") == ""
}

func namespaceInReport(ns, path string) string {
	if ns != "" {
		return fmt.Sprintf("namespace %q", ns)
	}
	if path != "" {
		return fmt.Sprintf("file %s", filepath.Base(path))
	}
	return "its namespace"
}

func fileInReport(f *fileInfo) string {
	// The core namespace has no name, and the "file X.go" fallback in
	// namespaceInReport was written for a file with no stem at all. Letting the
	// core fall into it would say "private to file client.go" about a
	// declaration every other core file may use — telling the reader something
	// the analyzer does not believe.
	if f.core {
		return "the core namespace"
	}
	return namespaceInReport(f.ns, f.path)
}

// reportsName reports whether the naming rule reaches a declaration at all.
//
// It reaches package-level declarations only: a member is already qualified by
// its type at every use, so a prefix would only stutter. It reaches an exported
// declaration when rules.naming.exported says so — inside the package an
// exported name is read as bare as any other, which is the reading the
// namespace mark exists for — and never reaches the core namespace, whose
// prefix is empty and so has nothing to require.
// reportsName reports whether the naming rule reaches this declaration.
//
// A member never carries a namespace: it is written inside its type and read
// through it. A method with a receiver is read through the receiver too, which
// names the unit its type belongs to. That answers the ownership question only
// while the two are the same unit. A method filed away from its type points the
// reader at a namespace that does not hold it, so the rule reaches it and asks
// for the namespace it is actually written in.
func (t *target) reportsName(opts Options) bool {
	if t.file.core || t.reportsToolchainName() {
		return false
	}
	switch {
	case t.contained:
		return false
	case t.kind == kindMethod:
		if !t.foreignMethod() {
			return false
		}
	case !t.renameable:
		return false
	}
	return !isExported(t.obj.Name()) || opts.NameExported
}

// reportsToolchainName reports whether the toolchain finds this declaration by its
// name, so that no rename can be asked of it. `go test` collects a test by
// name, and renaming TestLoad to userTestLoad leaves a function nothing runs.
//
// The match is looser than the toolchain's own, which also reads the signature
// and requires that TestXxx's Xxx not begin with a lowercase letter. Erring
// toward exempting is the safe direction here: exempting one name too many
// costs a rename nobody asked for, and exempting one too few is advice that
// breaks the build.
func (t *target) reportsToolchainName() bool {
	if t.kind != kindFunc || !strings.HasSuffix(t.file.path, "_test.go") {
		return false
	}
	name := t.obj.Name()
	for _, prefix := range [...]string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
