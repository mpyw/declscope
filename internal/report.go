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
	"github.com/mpyw/declscope/internal/scope"
)

// Rule names, used as the diagnostic category and as the baseline key.
const (
	ruleEscape        = "escape"
	ruleDemotion      = "demotion"
	ruleForeignMethod = "foreign-method"
)

// finding is a diagnostic that a target would produce, held back until its
// ignore directive and the baseline have been consulted.
type finding struct {
	rule    string
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
		findings := c.check(pass, opts, t)
		if t.dir.Ignore {
			// An ignore is unused only when nothing would have fired at all.
			// A baseline suppression still counts as the directive doing its
			// job, so the baseline is consulted after this.
			if len(findings) == 0 {
				pass.Reportf(t.dir.IgnorePos, "unused //declscope:ignore on %s", t.name())
			}
			continue
		}
		for _, f := range findings {
			if opts.Baseline.Has(f.key(pass)) {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:            f.pos,
				Category:       f.rule,
				Message:        f.msg,
				Related:        f.related,
				SuggestedFixes: f.fixes,
			})
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
		if t.dir.Ignore {
			continue
		}
		for _, f := range c.check(pass, opts, t) {
			out = append(out, f.key(pass))
		}
	}
	return out
}

func (c *collection) check(pass *analysis.Pass, opts Options, t *target) []finding {
	var out []finding
	switch t.scope {
	case scope.FilePrivate:
		if f, ok := c.checkEscape(pass, opts, t); ok {
			out = append(out, f)
		}
	case scope.PackageInternal:
		if f, ok := c.checkDemotion(pass, opts, t); ok {
			out = append(out, f)
		}
	case scope.Public:
	}
	if f, ok := c.checkForeignMethod(pass, opts, t); ok {
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

	f := finding{rule: ruleEscape, decl: t.name(), pos: t.ident.Pos()}
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
	// decisions and only they can resolve it. Offering to widen would have
	// -fix silently overwrite the directive they wrote; offering the rename
	// alone would be worse still, since the directive would keep the
	// declaration private and leave the new name lying about its reach.
	if t.dir.HasScope {
		return f, true
	}

	// A member is already namespaced by the type that owns it, so widening it
	// is a matter of intent, never of naming.
	if t.renameable && t.ownerNS != "" {
		if fix, ok := c.renameFix(pass, t, namespace.Qualify(t.obj.Name(), t.ownerNS),
			"promote to package-internal by prefixing with the namespace"); ok {
			f.fixes = append(f.fixes, fix)
		}
	}
	f.fixes = append(f.fixes, c.directiveFix(pass, t, scope.PackageInternal))
	return f, true
}

// checkDemotion reports a namespace-prefixed declaration whose every use stays
// inside its own namespace, so the prefix is claiming a reach it does not need.
func (c *collection) checkDemotion(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !opts.CheckDemotion || t.dir.HasScope || !t.renameable || t.ownerNS == "" {
		return finding{}, false
	}
	if !namespace.HasPrefix(t.obj.Name(), t.ownerNS) {
		return finding{}, false
	}
	refs := c.refs[t.obj]
	// No uses at all is a job for an unused-code linter, not this one.
	if len(refs) == 0 {
		return finding{}, false
	}
	for _, r := range refs {
		if r.file.key() != t.ownerKey {
			return finding{}, false
		}
	}

	short := namespace.Unqualify(t.obj.Name(), t.ownerNS)
	if short == t.obj.Name() {
		return finding{}, false
	}
	f := finding{
		rule: ruleDemotion,
		decl: t.name(),
		pos:  t.ident.Pos(),
		msg: fmt.Sprintf("%s %s is namespace-prefixed but is only used inside %s; drop the prefix or state the scope",
			t.kind, t.name(), describe(t.ownerNS, t.file.path)),
	}
	if fix, ok := c.renameFix(pass, t, short, "demote to file-private by dropping the namespace prefix"); ok {
		f.fixes = append(f.fixes, fix)
	}
	f.fixes = append(f.fixes, c.directiveFix(pass, t, scope.PackageInternal))
	return f, true
}

// checkForeignMethod reports an unexported method grown on a type that belongs
// to another namespace, which reaches into that namespace's internals from
// outside.
func (c *collection) checkForeignMethod(pass *analysis.Pass, opts Options, t *target) (finding, bool) {
	if !opts.CheckForeignMethods || t.kind != kindMethod || t.scope != scope.FilePrivate {
		return finding{}, false
	}
	if t.owner == "" || t.file.key() == t.ownerKey {
		return finding{}, false
	}
	return finding{
		rule: ruleForeignMethod,
		decl: t.name(),
		pos:  t.ident.Pos(),
		msg: fmt.Sprintf("unexported method %s is declared in %s but %s belongs to %s",
			t.name(), describeFile(t.file), t.owner, describe(t.ownerNS, "")),
		fixes: []analysis.SuggestedFix{c.directiveFix(pass, t, scope.PackageInternal)},
	}, true
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
