package internal

import (
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/measure"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// surveyed measures the package: its namespaces, every crossing between them,
// what became of each, and the tally behind each rule.
//
// It walks the same findings the report does, through one entry, in the same
// order, rather than asking the questions a second way. That is what keeps the
// two from drifting: a survey that recomputed "is this a violation" would be a
// second analyzer, and the first disagreement between them would read as a bug
// in one rather than as two different questions. What it adds is the state
// each finding ended in, which the report discards once it has decided whether
// to print, and the crossings themselves, which the report never names.
//
// The order matters beyond determinism. A rename fix reserves the name it
// claims, so whether a later one is offered depends on which targets came
// first; sorting exactly as report does is what makes Fixable the analyzer's
// own answer rather than an optimistic one.
//
//declscope:package // the survey entry, driven from analyzer.go
func (c *collection) surveyed(pass *analysis.Pass, opts Options) measure.Package {
	slices.SortStableFunc(c.targets, func(a, b *target) int {
		return comparePos(pass.Fset, a.ident.Pos(), b.ident.Pos())
	})

	out := measure.Package{
		Path:     pass.Pkg.Path(),
		Findings: countsForSurvey(c, opts),
	}

	for _, t := range c.targets {
		crossed := measure.EdgeState("")
		for _, f := range c.surveyedFindingsForReport(pass, opts, t) {
			bumpSurveyCount(out.Findings, f)
			switch f.Rule {
			case rule.Boundary:
				crossed = f.State
			case rule.Qualify:
				out.Names = append(out.Names, nameFindingForSurvey(t, f))
			}
		}
		out.Edges = append(out.Edges, edgesForSurvey(c, t, crossed)...)
	}

	out.Namespaces, out.AllCore = namespacesForSurvey(c, opts)
	out.Sort()
	return out
}

// edgesForSurvey turns one declaration's references from other namespaces into
// one edge per reaching namespace.
//
// The state comes from the boundary finding when there was one. When there was
// none, the crossing is still real, and why nothing was reported is itself the
// answer: a directive widened the declaration, the configured default did, or
// the rule is switched off.
func edgesForSurvey(c *collection, t *target, crossed measure.EdgeState) []measure.Edge {
	uses := map[string]int{}
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.file.key() {
			uses[r.file.namespaceForReport()]++
		}
	}
	if len(uses) == 0 {
		return nil
	}
	state := crossed
	if state == "" {
		state = stateOfUnreportedSurveyedEdge(t)
	}
	out := make([]measure.Edge, 0, len(uses))
	for from, n := range uses {
		out = append(out, measure.Edge{
			From:        from,
			To:          t.file.namespaceForReport(),
			Declaration: t.name(),
			Kind:        string(t.kind),
			Uses:        n,
			State:       state,
		})
	}
	return out
}

// stateOfUnreportedSurveyedEdge names why a crossing produced no finding.
//
// Declared is the one that matters: a directive says the declaration is
// shared, so the crossing is a decision somebody recorded. The test is not
// "bound by a directive", since //declscope:private is one of those too, and
// it is not "exported" either: a declaration widened by defaults.unexported
// reaches package scope with nothing written down about it, which is the same
// absence of a decision as an exported name. boundBy is the zero directive
// when the configured default supplied the scope, which is exactly that case.
func stateOfUnreportedSurveyedEdge(t *target) measure.EdgeState {
	switch {
	case t.scope == scope.Private:
		// Private, crossed, and nothing reported: the rule was not asked.
		// Nothing about this crossing has been decided.
		return measure.EdgeUnchecked
	case t.boundBy.HasScope && t.boundBy.Scope == scope.PackageInternal:
		return measure.EdgeDeclared
	default:
		return measure.EdgeOpen
	}
}

// nameFindingForSurvey records one naming finding against the declaration it
// was found on.
func nameFindingForSurvey(t *target, f measure.Finding) measure.NameFinding {
	return measure.NameFinding{
		Namespace:   t.file.namespaceForReport(),
		File:        t.file.path,
		Declaration: f.Declaration,
		Kind:        string(t.kind),
		Exported:    isExported(t.obj.Name()),
		State:       nameStateForSurvey(f.State),
		Fixable:     f.Fixable,
	}
}

func nameStateForSurvey(state measure.EdgeState) measure.NameState {
	switch state {
	case measure.EdgeIgnored:
		return measure.NameExempt
	case measure.EdgeBaselined:
		return measure.NameBaselined
	default:
		return measure.NameReported
	}
}

// namespacesForSurvey counts, per namespace, what the two rules divide by.
//
// The naming denominator is target.reportsName and nothing else. Spelling that
// predicate out a second time here would let a change to the rule move a
// survey number without moving a diagnostic, which is the drift this command
// exists not to introduce.
func namespacesForSurvey(c *collection, opts Options) ([]measure.Namespace, bool) {
	byKey := map[string]*measure.Namespace{}
	order := make([]string, 0, len(c.files))
	for _, fi := range c.files {
		ns, ok := byKey[fi.key()]
		if !ok {
			ns = &measure.Namespace{Name: fi.namespaceForReport(), Core: fi.core}
			byKey[fi.key()] = ns
			order = append(order, fi.key())
		}
		ns.Files = append(ns.Files, fi.path)
	}
	for _, t := range c.targets {
		ns, ok := byKey[t.file.key()]
		if !ok {
			continue
		}
		ns.Declarations++
		if t.reportsName(opts) {
			ns.QualifyTargets++
		}
	}

	out := make([]measure.Namespace, 0, len(order))
	allCore := len(order) > 0
	for _, key := range order {
		ns := byKey[key]
		if !ns.Core {
			allCore = false
		}
		out = append(out, *ns)
	}
	return out, allCore
}

// countsForSurvey seeds one tally per rule, with whether the rule was in force
// at all. A rule that is switched off counts zero for a reason that has
// nothing to do with the code, and saying so is the whole point of reporting a
// tally beside the checks.
func countsForSurvey(c *collection, opts Options) map[rule.Rule]measure.Count {
	return map[rule.Rule]measure.Count{
		rule.Boundary: {Asked: !opts.AllowBoundary, Keyable: true},
		rule.Qualify:  {Asked: opts.Qualify.Applies(c.namespaces), Keyable: true},
		rule.Surplus:  {Asked: !opts.AllowSurplus, Keyable: true},
	}
}

func bumpSurveyCount(counts map[rule.Rule]measure.Count, f measure.Finding) {
	count := counts[f.Rule]
	count.Found++
	switch f.State {
	case measure.EdgeIgnored:
		count.Ignored++
	case measure.EdgeBaselined:
		count.Baselined++
	default:
		count.Reported++
	}
	counts[f.Rule] = count
}
