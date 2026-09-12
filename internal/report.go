//declscope:namespace analyzer

package internal

import (
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/scope"
)

// finding is a diagnostic that a target would produce, held back until its
// ignore directive has been consulted.
type finding struct {
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
			if len(findings) == 0 {
				pass.Reportf(t.dir.IgnorePos, "unused //declscope:ignore on %s", t.name())
			}
			continue
		}
		for _, f := range findings {
			pass.Report(analysis.Diagnostic{
				Pos:            f.pos,
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

	f := finding{pos: t.ident.Pos()}
	if t.owner != "" {
		f.msg = fmt.Sprintf("%s %s is private to %s, but is used from %s",
			t.kind, t.name(), describe(t.ownerNS, t.file.path), describeFile(offenders[0].file))
	} else {
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
		pos: t.ident.Pos(),
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
		pos: t.ident.Pos(),
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
func (c *collection) directiveFix(pass *analysis.Pass, t *target, s scope.Scope) analysis.SuggestedFix {
	col := pass.Fset.Position(t.anchor).Column
	indent := strings.Repeat("\t", max(col-1, 0))
	return analysis.SuggestedFix{
		Message: fmt.Sprintf("add %s to %s", s.Directive(), t.name()),
		TextEdits: []analysis.TextEdit{{
			Pos:     t.anchor,
			End:     t.anchor,
			NewText: []byte(s.Directive() + "\n" + indent),
		}},
	}
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
