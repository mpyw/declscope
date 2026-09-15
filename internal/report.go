package internal

import (
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/directive"
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

	for _, t := range c.targets {
		for _, f := range c.checkForReport(pass, opts, t) {
			// Ignores are consulted before the baseline: a suppression the
			// baseline would also have absorbed still counts as the directive
			// doing its job.
			if c.silencedByIgnore(t, f.rule) {
				continue
			}
			if opts.Baseline.Has(f.key(pass, t)) {
				continue
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
}

// nsNameInReport spells the namespace for the baseline, which is the one place it has
// to be written down rather than compared.
//
// A namespace derived from a file name is normalized to alphanumerics, and one
// written with //declscope:namespace must be an unexported identifier, so a
// parenthesis can appear in neither. That leaves "(core)" free for the core,
// whose prefix is empty and whose files all share it, and free for the file
// with no stem at all — which has no namespace either, and must not share a
// key with every other such file.
func (f *fileInfo) nsNameInReport() string {
	if f.core {
		return "(core)"
	}
	if f.ns != "" {
		return f.ns
	}
	return "(file " + filepath.Base(f.path) + ")"
}

// key identifies the finding for the baseline, independently of position.
func (f reportedFinding) key(pass *analysis.Pass, t *target) baseline.Key {
	return baseline.Key{
		Package:   pass.Pkg.Path(),
		Rule:      f.rule,
		Namespace: t.file.nsNameInReport(),
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
		for _, f := range c.checkForReport(pass, opts, t) {
			if c.silencedByIgnore(t, f.rule) {
				continue
			}
			out = append(out, f.key(pass, t))
		}
	}
	return out
}

func (c *collection) checkForReport(pass *analysis.Pass, opts Options, t *target) []reportedFinding {
	var out []reportedFinding
	// An exported declaration resolves to package scope unless a directive
	// narrows it, so the one test below covers both: what is reachable from
	// outside carries no boundary, and what an author narrowed does.
	if t.scope == scope.Private {
		if f, ok := c.checkBoundaryForReport(pass, opts, t); ok {
			out = append(out, f)
		}
	}
	if f, ok := c.checkQualifyForReport(pass, opts, t); ok {
		out = append(out, f)
	}
	// The judgment and the wording both live in surplus.go; only the finding
	// is assembled here, so that the ignore chain and the baseline treat the
	// rule like any other.
	if pos, msg, ok := c.checkSurplus(pass, opts, t); ok {
		out = append(out, reportedFinding{rule: rule.Surplus, decl: t.name(), pos: pos, msg: msg})
	}
	return out
}

// checkBoundaryForReport reports a declaration that is private to its namespace but is
// referenced from outside it.
func (c *collection) checkBoundaryForReport(pass *analysis.Pass, opts Options, t *target) (reportedFinding, bool) {
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
	f.fixes = append(f.fixes, directiveFixInReport(pass, t, scope.PackageInternal))
	return f, true
}

// checkQualifyForReport requires a package-level declaration to carry its namespace
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
// to never: the convention is opt-in. See Mode and DefaultOptions.
func (c *collection) checkQualifyForReport(pass *analysis.Pass, opts Options, t *target) (reportedFinding, bool) {
	if !opts.Qualify.Applies(c.namespaces) || !t.namedInReport(opts) {
		return reportedFinding{}, false
	}
	// A namespace is always an identity, but not always a prefix: 2fa.go
	// bounds its declarations like any other file, yet no identifier can
	// start with a digit. Containment alone could be satisfied there, by
	// spelling the namespace later in the name, but the fix could not be,
	// so the rule stays out rather than report what it cannot remedy.
	if !namespace.CanPrefix(t.file.ns) {
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
	// main is a name the toolchain requires, so nothing can be asked of it.
	if t.kind == kindFunc && name == "main" && pass.Pkg.Name() == "main" {
		return reportedFinding{}, false
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

// directiveFixInReport inserts an explicit scope directive above the declaration.
//
// A directive only binds to a declaration when it sits on its own line above
// it, so a declaration that shares a line with something else — a field of a
// single-line struct, for instance — first has to be broken onto a line of its
// own. The formatter applied to the fixed file restores the indentation.
func directiveFixInReport(pass *analysis.Pass, t *target, s scope.Scope) analysis.SuggestedFix {
	var text string
	if atLineStartForReport(pass, t.anchor) {
		col := pass.Fset.Position(t.anchor).Column
		text = s.Directive() + "\n" + strings.Repeat("\t", max(col-1, 0))
	} else {
		text = "\n" + s.Directive() + "\n"
	}
	return analysis.SuggestedFix{
		Message: fmt.Sprintf("add %s to %s", s.Directive(), t.name()),
		TextEdits: []analysis.TextEdit{{
			Pos:     t.anchor,
			End:     t.anchor,
			NewText: []byte(text),
		}},
	}
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

// namedInReport reports whether the naming rule reaches a declaration at all.
//
// It reaches package-level declarations only: a member is already qualified by
// its type at every use, so a prefix would only stutter. It reaches an exported
// declaration when rules.naming.exported says so — inside the package an
// exported name is read as bare as any other, which is the reading the
// namespace mark exists for — and never reaches the core namespace, whose
// prefix is empty and so has nothing to require.
// namedInReport reports whether the naming rule reaches this declaration.
//
// A member never carries a namespace: it is written inside its type and read
// through it. A method with a receiver is read through the receiver too, which
// names the unit its type belongs to. That answers the ownership question only
// while the two are the same unit. A method filed away from its type points the
// reader at a namespace that does not hold it, so the rule reaches it and asks
// for the namespace it is actually written in.
func (t *target) namedInReport(opts Options) bool {
	if t.file.core || t.toolchainNameInReport() {
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

// toolchainNameInReport reports whether the toolchain finds this declaration by its
// name, so that no rename can be asked of it. `go test` collects a test by
// name, and renaming TestLoad to userTestLoad leaves a function nothing runs.
//
// The match is looser than the toolchain's own, which also reads the signature
// and requires that TestXxx's Xxx not begin with a lowercase letter. Erring
// toward exempting is the safe direction here: exempting one name too many
// costs a rename nobody asked for, and exempting one too few is advice that
// breaks the build.
func (t *target) toolchainNameInReport() bool {
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
