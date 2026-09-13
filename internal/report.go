//declscope:namespace analyzer

package internal

import (
	"cmp"
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

// finding is a diagnostic that a target would produce, held back until its
// ignore directives and the baseline have been consulted.
type finding struct {
	rule    rule.Rule
	decl    string
	pos     token.Pos
	msg     string
	related []analysis.RelatedInformation
	fixes   []analysis.SuggestedFix
}

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
		for _, f := range c.check(pass, opts, t) {
			// Ignores are consulted before the baseline: a suppression the
			// baseline would also have absorbed still counts as the directive
			// doing its job.
			if c.silenced(t, f.rule) {
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
	c.reportUnusedScopes(pass)

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
		if c.silencesFile(c.fileAt(pass, p.Pos), rule.Directive) {
			continue
		}
		surviving = append(surviving, p)
	}
	c.problems = surviving

	// Only now, with every ignore that silenced something marked used. An
	// ignore report is itself a directive problem, so it is appended rather
	// than reported directly, and joins the same ordering — and the same
	// silencing, which is why the filter runs again over the tail. Running it
	// once would leave the one report nothing could answer.
	c.reportUnusedIgnores(pass)
	kept := c.problems[:0]
	for _, p := range c.problems {
		if c.silencesFile(c.fileAt(pass, p.Pos), rule.Directive) {
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

// comparePos orders positions by file name, then by offset within the file.
func comparePos(fset *token.FileSet, a, b token.Pos) int {
	pa, pb := fset.Position(a), fset.Position(b)
	if c := strings.Compare(pa.Filename, pb.Filename); c != 0 {
		return c
	}
	return cmp.Compare(pa.Offset, pb.Offset)
}

// key identifies the finding for the baseline, independently of position.
func (f finding) key(pass *analysis.Pass, t *target) baseline.Key {
	return baseline.Key{
		Package:   pass.Pkg.Path(),
		Rule:      f.rule,
		Namespace: t.ownerFile.nsName(),
		Decl:      f.decl,
	}
}

// keys returns every violation the pass would report, ignoring the baseline.
// It is what regenerating a baseline records.
func (c *collection) keys(pass *analysis.Pass, opts Options) []baseline.Key {
	var out []baseline.Key
	for _, t := range c.targets {
		for _, f := range c.check(pass, opts, t) {
			if c.silenced(t, f.rule) {
				continue
			}
			out = append(out, f.key(pass, t))
		}
	}
	return out
}

func (c *collection) check(pass *analysis.Pass, opts Options, t *target) []finding {
	var out []finding
	// An exported declaration resolves to package scope unless a directive
	// narrows it, so the one test below covers both: what is reachable from
	// outside carries no boundary, and what an author narrowed does.
	if t.scope == scope.Private {
		if f, ok := c.checkBoundary(pass, opts, t); ok {
			out = append(out, f)
		}
	}
	if f, ok := c.checkQualify(pass, opts, t); ok {
		out = append(out, f)
	}
	if f, ok := c.checkUnqualify(pass, opts, t); ok {
		out = append(out, f)
	}
	return out
}

// checkBoundary reports a declaration that is private to its namespace but is
// referenced from outside it.
func (c *collection) checkBoundary(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	var offenders []ref
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.ownerKey {
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
	case levelDecl:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), describeFile(offenders[0].file))
	case levelContainer:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s on %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), t.owner, describeFile(offenders[0].file))
	case levelFile:
		f.msg = fmt.Sprintf("%s %s is declared %s by the file's %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), describeFile(offenders[0].file))
	default:
		f.msg = fmt.Sprintf("%s %s is private to %s, but is used from %s",
			t.kind, t.name(), describeFile(t.ownerFile), describeFile(offenders[0].file))
	}
	for _, r := range offenders {
		f.related = append(f.related, analysis.RelatedInformation{
			Pos:     r.ident.Pos(),
			End:     r.ident.End(),
			Message: fmt.Sprintf("used here, in %s", describeFile(r.file)),
		})
	}

	// When the author stated the scope ON THE DECLARATION, the conflict is
	// between two explicit decisions and only they can resolve it: widening
	// would have -fix silently overwrite the directive they wrote.
	//
	// A scope inherited from the containing type or from the file is a default,
	// and a directive inserted on the declaration states an exception to it
	// rather than overwriting a decision — but inserting one would also leave
	// the outer directive binding one declaration fewer, which can make it
	// unused. Offering the fix there would produce a diagnostic that did not
	// exist before, so it is withheld at every level that supplied the scope.
	if t.boundAt != levelDefault {
		return f, true
	}
	f.fixes = append(f.fixes, c.directiveFix(pass, t, scope.PackageInternal))
	return f, true
}

// checkQualify requires a package-level declaration to carry its namespace as
// a prefix.
//
// The prefix grants nothing — reach is stated with a directive — so this is
// purely an ownership label, making the owning unit legible at every use site
// and in every stack trace and grep result.
//
// Whether it applies at all depends on rules.qualify, which defaults to
// requiring the label only once a package has a second namespace to
// distinguish. See Mode.
func (c *collection) checkQualify(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !opts.Qualify.Applies(c.namespaces) || !named(opts, t) {
		return finding{}, false
	}
	// A namespace is always an identity, but not always a label: 2fa.go
	// bounds its declarations like any other file, yet no identifier can
	// start with a digit, so there is no prefix to ask for.
	if !namespace.IsLabel(t.ownerNS) {
		return finding{}, false
	}
	name := t.obj.Name()
	if namespace.HasPrefix(name, t.ownerNS) {
		return finding{}, false
	}
	// main is a name the toolchain requires, so no label can be asked of it.
	if t.kind == kindFunc && name == "main" && pass.Pkg.Name() == "main" {
		return finding{}, false
	}

	f := finding{
		rule: rule.Qualify,
		decl: name,
		pos:  t.ident.Pos(),
		msg: fmt.Sprintf("%s %s does not carry the prefix of %s; rename it to %s",
			t.kind, name, describe(t.ownerNS, t.file.path), namespace.Qualify(name, t.ownerNS)),
	}
	if fix, ok := c.renameFix(pass, t, namespace.Qualify(name, t.ownerNS),
		"label it with its namespace"); ok {
		f.fixes = append(f.fixes, fix)
	}
	return f, true
}

// checkUnqualify is the mirror of checkQualify: where the label is not required,
// it must not be there either.
//
// Enabling it asserts that in this codebase a namespace prefix always means
// the label and never part of the concept, since nothing in the name can tell
// userID-the-label from userID-the-word.
func (c *collection) checkUnqualify(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !opts.Unqualify || !named(opts, t) || !namespace.IsLabel(t.ownerNS) {
		return finding{}, false
	}
	if opts.Qualify.Applies(c.namespaces) {
		return finding{}, false
	}
	name := t.obj.Name()
	if !namespace.HasPrefix(name, t.ownerNS) {
		return finding{}, false
	}
	// A name identical to the namespace carries no label to drop. The
	// causality usually runs the other way there: user.go is named after the
	// user it declares, not the other way about. qualify still accepts such a
	// name, since the owning unit is legible from it, but there is nothing
	// here for unqualify to strip. The label is matched ignoring case, so the
	// exemption is too: userId in user_id.go is the namespace, spelled by
	// someone who did not know how the linter would spell it.
	if strings.EqualFold(name, t.ownerNS) {
		return finding{}, false
	}

	// Not being able to spell the new name is a limit of the fix, not a reason
	// to let the label stand: the violation is reported either way, and only
	// the suggestion is withheld.
	short, why := namespace.Unqualify(name, t.ownerNS)
	f := finding{rule: rule.Unqualify, decl: name, pos: t.ident.Pos()}
	if short == "" {
		f.msg = fmt.Sprintf("%s %s carries the label of %s, which is not required here, but %s; rename it by hand",
			t.kind, name, describe(t.ownerNS, t.file.path), why)
		return f, true
	}

	f.msg = fmt.Sprintf("%s %s carries the label of %s, which is not required here; rename it to %s",
		t.kind, name, describe(t.ownerNS, t.file.path), short)
	if fix, ok := c.renameFix(pass, t, short, "drop the namespace label"); ok {
		f.fixes = append(f.fixes, fix)
	}
	return f, true
}

// renameFix rewrites every ident naming the target. All of them are inside the
// package, so the edits stay within the pass.
//
// The fix is offered only when renameSafe can prove it changes nothing but
// the spelling; the diagnostic is reported either way. Whatever it renames to
// is reserved for the rest of the pass, since a later fix checking the same
// pre-fix state would otherwise find the name still free.
func (c *collection) renameFix(pass *analysis.Pass, t *target, newName, message string) (analysis.SuggestedFix, bool) {
	if newName == t.obj.Name() {
		return analysis.SuggestedFix{}, false
	}
	idents := c.idents[t.obj]
	if len(idents) == 0 {
		return analysis.SuggestedFix{}, false
	}
	if !c.renameSafe(pass, t, newName) {
		return analysis.SuggestedFix{}, false
	}
	c.reserve(newName)
	edits := make([]analysis.TextEdit, 0, len(idents))
	for _, id := range idents {
		edits = append(edits, analysis.TextEdit{Pos: id.Pos(), End: id.End(), NewText: []byte(newName)})
	}
	return analysis.SuggestedFix{
		Message:   fmt.Sprintf("rename %s to %s (%s)", t.obj.Name(), newName, message),
		TextEdits: edits,
	}, true
}

// directiveFix inserts an explicit scope directive above the declaration.
//
// A directive only binds to a declaration when it sits on its own line above
// it, so a declaration that shares a line with something else — a field of a
// single-line struct, for instance — first has to be broken onto a line of its
// own. The formatter applied to the fixed file restores the indentation.
func (c *collection) directiveFix(pass *analysis.Pass, t *target, s scope.Scope) analysis.SuggestedFix {
	var text string
	if c.startsLine(pass, t.anchor) {
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

// startsLine reports whether pos is preceded on its line by nothing but
// whitespace. It fails safe: an unreadable file is treated as not starting a
// line, which yields an extra line break rather than a misplaced directive.
func (c *collection) startsLine(pass *analysis.Pass, pos token.Pos) bool {
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

func describe(ns, path string) string {
	if ns != "" {
		return fmt.Sprintf("namespace %q", ns)
	}
	if path != "" {
		return fmt.Sprintf("file %s", filepath.Base(path))
	}
	return "its namespace"
}

func describeFile(f *fileInfo) string {
	// The core namespace has no name, and the "file X.go" fallback in describe
	// was written for a file with no stem at all. Letting the core fall into it
	// would say "private to file client.go" about a declaration every other core
	// file may use — telling the reader something the analyzer does not believe.
	if f.core {
		return "the core namespace"
	}
	return describe(f.ns, f.path)
}

// named reports whether the naming rules reach a declaration at all.
//
// They reach package-level declarations only: a member is already qualified by
// its type at every use, so a label would only stutter. They reach an exported
// declaration when rules.exportedLabels says so — inside the package an
// exported name is read as bare as any other, which is the reading the label
// exists for — and never reach the core namespace, whose label is empty and so
// has no prefix to require or to drop.
func named(opts Options, t *target) bool {
	if !t.renameable || t.ownerFile.core {
		return false
	}
	return !isExported(t.obj.Name()) || opts.ExportedLabels
}
