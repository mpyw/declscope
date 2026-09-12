//declscope:namespace analyzer

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
	slices.SortStableFunc(c.targets, func(a, b *target) int {
		return int(a.ident.Pos() - b.ident.Pos())
	})

	for _, t := range c.targets {
		used := make([]bool, len(t.dir.Ignores))
		for _, f := range c.check(pass, opts, t) {
			// Ignores are consulted before the baseline: a suppression that
			// the baseline would also have absorbed still counts as the
			// directive doing its job. Both levels are consulted, and both
			// marked, so neither is reported unused for overlapping.
			declIgnored := ignored(t.dir.Ignores, f.rule, used)
			fileIgnored := ignored(t.file.ignores, f.rule, t.file.ignoresUsed)
			if declIgnored || fileIgnored {
				continue
			}
			if opts.Baseline.Has(f.key(pass)) {
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
		for i, ig := range t.dir.Ignores {
			if !used[i] {
				pass.Reportf(ig.Pos, "unused %s on %s", ig, t.name())
			}
		}
	}

	for _, fi := range c.files {
		for i, ig := range fi.ignores {
			if !fi.ignoresUsed[i] {
				pass.Reportf(ig.Pos, "unused file-level %s", ig)
			}
		}
	}

	slices.SortStableFunc(c.problems, func(a, b directive.Problem) int { return int(a.Pos - b.Pos) })
	for _, p := range c.problems {
		pass.Reportf(p.Pos, "%s", p.Msg)
	}
}

// key identifies the finding for the baseline, independently of position.
func (f finding) key(pass *analysis.Pass) baseline.Key {
	return baseline.Key{Package: pass.Pkg.Path(), Rule: f.rule, Decl: f.decl}
}

// keys returns every violation the pass would report, ignoring the baseline.
// It is what regenerating a baseline records.
func (c *collection) keys(pass *analysis.Pass, opts Options) []baseline.Key {
	var out []baseline.Key
	for _, t := range c.targets {
		used := make([]bool, len(t.dir.Ignores))
		for _, f := range c.check(pass, opts, t) {
			declIgnored := ignored(t.dir.Ignores, f.rule, used)
			fileIgnored := ignored(t.file.ignores, f.rule, t.file.ignoresUsed)
			if declIgnored || fileIgnored {
				continue
			}
			out = append(out, f.key(pass))
		}
	}
	return out
}

// ignored reports whether any directive silences r, marking every directive
// that does as used. All of them are marked, not just the first, so that
// overlapping directives are not reported as unused.
func ignored(ignores []directive.Ignore, r rule.Rule, used []bool) bool {
	hit := false
	for i, ig := range ignores {
		if ig.Covers(r) {
			used[i] = true
			hit = true
		}
	}
	return hit
}

func (c *collection) check(pass *analysis.Pass, opts Options, t *target) []finding {
	var out []finding
	if t.scope == scope.FilePrivate {
		if f, ok := c.checkEscape(pass, opts, t); ok {
			out = append(out, f)
		}
	}
	if f, ok := c.checkPromote(pass, opts, t); ok {
		out = append(out, f)
	}
	if f, ok := c.checkDemote(pass, opts, t); ok {
		out = append(out, f)
	}
	return out
}

// checkEscape reports a declaration that is private to its namespace but is
// referenced from outside it.
func (c *collection) checkEscape(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	var offenders []ref
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.ownerKey {
			offenders = append(offenders, r)
		}
	}
	if len(offenders) == 0 {
		return finding{}, false
	}

	f := finding{rule: rule.Escape, decl: t.name(), pos: t.ident.Pos()}
	switch {
	case t.dir.HasScope:
		f.msg = fmt.Sprintf("%s %s is declared %s by %s, but is used from %s",
			t.kind, t.name(), t.scope, t.scope.Directive(), describeFile(offenders[0].file))
	case t.owner != "":
		f.msg = fmt.Sprintf("%s %s is private to %s, but is used from %s",
			t.kind, t.name(), describe(t.ownerNS, t.file.path), describeFile(offenders[0].file))
	default:
		f.msg = fmt.Sprintf("%s %s is file-private to %s, but is used from %s",
			t.kind, t.name(), describe(t.ownerNS, t.file.path), describeFile(offenders[0].file))
	}
	for _, r := range offenders {
		f.related = append(f.related, analysis.RelatedInformation{
			Pos:     r.ident.Pos(),
			End:     r.ident.End(),
			Message: fmt.Sprintf("used here, in %s", describeFile(r.file)),
		})
	}

	// When the author stated the scope, the conflict is between two explicit
	// decisions and only they can resolve it: widening would have -fix
	// silently overwrite the directive they wrote.
	if t.dir.HasScope {
		return f, true
	}
	f.fixes = append(f.fixes, c.directiveFix(pass, t, scope.PackageInternal))
	return f, true
}

// checkPromote requires an unexported package-level declaration to carry its
// namespace as a prefix.
//
// The prefix grants nothing — reach is stated with a directive — so this is
// purely an ownership label, making the owning unit legible at every use site
// and in every stack trace and grep result.
//
// Whether it applies at all depends on rules.promote, which defaults to
// requiring the label only once a package has a second namespace to
// distinguish. See PromoteMode.
func (c *collection) checkPromote(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !opts.Promote.required(c.namespaces) || !t.renameable || t.ownerNS == "" {
		return finding{}, false
	}
	name := t.obj.Name()
	if isExported(name) || namespace.HasPrefix(name, t.ownerNS) {
		return finding{}, false
	}
	// main is spelled by the toolchain, not by us.
	if t.kind == kindFunc && name == "main" && pass.Pkg.Name() == "main" {
		return finding{}, false
	}

	f := finding{
		rule: rule.Promote,
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

// checkDemote is the mirror of checkPromote: where the label is not required,
// it must not be there either.
//
// Enabling it asserts that in this codebase a namespace prefix always means
// the label and never part of the concept, since nothing in the name can tell
// userID-the-label from userID-the-word.
func (c *collection) checkDemote(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !opts.CheckDemote || !t.renameable || t.ownerNS == "" {
		return finding{}, false
	}
	if opts.Promote.required(c.namespaces) {
		return finding{}, false
	}
	name := t.obj.Name()
	if isExported(name) || !namespace.HasPrefix(name, t.ownerNS) {
		return finding{}, false
	}
	// A name identical to the namespace carries no label to drop. The
	// causality usually runs the other way there: user.go is named after the
	// user it declares, not the other way about. promote still accepts such a
	// name, since the owning unit is legible from it, but there is nothing
	// here for demote to strip.
	if name == t.ownerNS {
		return finding{}, false
	}

	// Not being able to spell the new name is a limit of the fix, not a reason
	// to let the label stand: the violation is reported either way, and only
	// the suggestion is withheld.
	short, why := namespace.Unqualify(name, t.ownerNS)
	f := finding{rule: rule.Demote, decl: name, pos: t.ident.Pos()}
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
func (c *collection) renameFix(pass *analysis.Pass, t *target, newName, message string) (analysis.SuggestedFix, bool) {
	if newName == t.obj.Name() {
		return analysis.SuggestedFix{}, false
	}
	// Renaming into a name the package already uses would not compile.
	if pass.Pkg.Scope().Lookup(newName) != nil {
		return analysis.SuggestedFix{}, false
	}
	idents := c.idents[t.obj]
	if len(idents) == 0 {
		return analysis.SuggestedFix{}, false
	}
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
	return describe(f.ns, f.path)
}
