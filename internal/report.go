package internal

import (
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
)

// pendingReport is one target's surviving findings, held until every target
// has been judged. A fix is decided against the pre-fix source, so what one
// fix is about to write can only be known once they all exist.
type pendingReport struct {
	t        *target
	findings []finding
}

// widensReport reports whether this finding's fix widens the declaration to
// shared scope, and with it every member the declaration contains. Only a
// type contains members, and only an inserted directive widens: a rename
// changes no reach at all.
func (f finding) widensReport(t *target) bool {
	return f.rule == rule.Boundary && len(f.fixes) > 0 && t.kind == kindType && !t.contained
}

// subsumedByReport reports whether the fix a member's finding carries is
// already going to be written by the fix on its type, in the same run.
//
// Every fix is decided against the pre-fix source and none of them can see the
// others, so two can converge — the shape spec/rename_siblings.fsl already
// models for renames. A directive inserted on a type reaches the type's
// members, which leaves a directive inserted on a member in the same run
// binding nothing: the unused rule then reports what -fix just wrote, and a
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
func subsumedByReport(t *target, f finding, widened map[types.Object]bool) bool {
	return f.rule == rule.Boundary && t.contained && t.ownerObj != nil && widened[t.ownerObj]
}

// report renders every diagnostic of the pass.
//
//declscope:shared // the analyzer's reporting entry, driven from analyzer.go
func (c *collection) report(pass *analysis.Pass, opts Options) {
	// Every surviving finding is collected before any of them is reported,
	// because one fix can subsume another and no fix can see the edits of the
	// others. widensReport marks what one is about to widen; subsumedByReport
	// spells out why.
	pending := make([]pendingReport, 0, len(c.targets))
	widened := make(map[types.Object]bool)
	for _, t := range c.targets {
		p := pendingReport{t: t}
		for _, f := range c.findingsOf(pass, opts, t) {
			if f.settled(pass, opts) {
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
	c.reportUnusedScopeSites(pass, opts, widened)

	// Directive hygiene carries rules like every other check, so that
	// //declscope:ignore unused or directive can silence one. A report with
	// no rule is one nothing could answer.
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
	c.problems = c.problemsLeftByIgnores(pass)

	// Only now, with every ignore that silenced something marked used. An
	// ignore report is itself a problem, so it is appended rather than
	// reported directly, and joins the same ordering — and the same
	// silencing, which is why the filter runs again over the tail. Running it
	// once would leave the one report nothing could answer.
	c.reportUnusedIgnores(pass, opts)
	c.problems = c.problemsLeftByIgnores(pass)

	for _, p := range c.problems {
		pass.Report(analysis.Diagnostic{
			Pos:            p.Pos,
			Category:       string(p.Rule),
			Message:        p.Msg,
			SuggestedFixes: p.Fixes,
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

// keysForReport returns every violation the pass would report, ignoring the baseline.
// It is what regenerating a baseline records.
//
//declscope:shared // the baseline regeneration entry, driven from analyzer.go
func (c *collection) keysForReport(pass *analysis.Pass, opts Options) []baseline.Key {
	var out []baseline.Key
	for _, t := range c.targets {
		for _, f := range c.findingsOf(pass, opts, t) {
			if c.silencedByIgnore(t, f.rule) {
				continue
			}
			out = append(out, f.key(pass, t))
		}
	}
	return out
}
