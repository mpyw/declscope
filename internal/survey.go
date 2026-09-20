package internal

import (
	"path/filepath"
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
		Findings: countsForSurvey(pass, c, opts),
	}

	for _, t := range c.targets {
		crossed := measure.FindingState("")
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

	// The directive and filter rules are settled once every other finding has
	// been seen, exactly as the report settles them.
	out.Findings[rule.Directive], out.Findings[rule.Filter] = c.surveyedProblemsForReport(pass)

	out.Namespaces, out.AllCore = namespacesForSurvey(pass, c, opts)
	return out.Sorted()
}

// edgesForSurvey turns one declaration's references from other namespaces into
// one edge per reaching namespace.
//
// The state comes from the boundary finding when there was one. When there was
// none, the crossing is still real, and why nothing was reported is itself the
// answer: a directive widened the declaration, the configured default did, or
// the rule is switched off.
func edgesForSurvey(c *collection, t *target, crossed measure.FindingState) []measure.Edge {
	uses := map[string]int{}
	for _, r := range c.refs[t.obj] {
		if r.file.key() != t.file.key() {
			uses[r.file.namespaceForReport()]++
		}
	}
	if len(uses) == 0 {
		return nil
	}
	state := edgeStateForSurvey(crossed)
	if crossed == "" {
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
// shared, so the crossing is a decision somebody recorded. Three things it is
// not. Not "bound by a directive", since //declscope:private is one of those
// too. Not "unexported", since a declaration widened by defaults.unexported
// reaches package scope with nothing written down about it, which is the same
// absence of a decision as an exported name. And not "the directive that
// supplied the scope was used": that is a question about the directive, and
// this is a question about the declaration. One file-level
// //declscope:package can decide for an unexported name and decide nothing
// for the exported one beside it, and target.decided is that per-declaration
// answer, taken where the scope was resolved.
func stateOfUnreportedSurveyedEdge(t *target) measure.EdgeState {
	switch {
	case t.scope == scope.Private:
		// Private, crossed, and nothing reported: the rule was not asked.
		// Nothing about this crossing has been decided.
		return measure.EdgeUnchecked
	case t.decided && t.boundBy.Scope == scope.PackageInternal:
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
		File:        filepath.Base(t.file.path),
		Declaration: f.Declaration,
		Kind:        string(t.kind),
		Exported:    isExported(t.obj.Name()),
		State:       nameStateForSurvey(f.State),
		Fixable:     f.Fixable,
	}
}

// edgeStateForSurvey carries a finding's outcome onto the crossing it was
// found on. The three states a finding can hold are the three a crossing
// shares with it; the other three say why there was no finding at all.
func edgeStateForSurvey(state measure.FindingState) measure.EdgeState {
	switch state {
	case measure.FindingIgnored:
		return measure.EdgeIgnored
	case measure.FindingBaselined:
		return measure.EdgeBaselined
	default:
		return measure.EdgeReported
	}
}

func nameStateForSurvey(state measure.FindingState) measure.NameState {
	switch state {
	case measure.FindingIgnored:
		return measure.NameExempt
	case measure.FindingBaselined:
		return measure.NameBaselined
	default:
		return measure.NameReported
	}
}

// namespacesForSurvey counts, per namespace, what the two rules divide by.
//
// The naming denominator is qualifyExaminesForReport, which is the whole gate
// the rule itself applies. Spelling any part of it out a second time here
// would let a change to the rule move a survey number without moving a
// diagnostic, which is the drift this command exists not to introduce.
func namespacesForSurvey(pass *analysis.Pass, c *collection, opts Options) ([]measure.Namespace, bool) {
	byKey := map[string]*measure.Namespace{}
	order := make([]string, 0, len(c.files))
	for _, fi := range c.files {
		ns, ok := byKey[fi.key()]
		if !ok {
			ns = &measure.Namespace{Name: fi.namespaceForReport(), Core: fi.core}
			byKey[fi.key()] = ns
			order = append(order, fi.key())
		}
		// The base name, not the path: every file of a package sits in one
		// directory, so the stem identifies it, and the report stays the same
		// whoever runs it and from wherever.
		ns.Files = append(ns.Files, filepath.Base(fi.path))
	}
	for _, t := range c.targets {
		ns, ok := byKey[t.file.key()]
		if !ok {
			continue
		}
		ns.Declarations++
		if c.qualifyExaminesForReport(pass, opts, t) {
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
func countsForSurvey(pass *analysis.Pass, c *collection, opts Options) map[rule.Rule]measure.Count {
	return map[rule.Rule]measure.Count{
		rule.Boundary: {Asked: !opts.AllowBoundary, Keyable: true},
		rule.Qualify:  {Asked: opts.Qualify.Applies(c.namespaces), Keyable: true},
		// Not !opts.AllowSurplus: the rule also stands itself down for a
		// package this pass cannot see every file of, and a count of zero
		// there is the "zero for a reason that is not the code" this column
		// exists to rule out.
		rule.Surplus: {Asked: !opts.AllowSurplus && c.surplusSeesEveryFile(pass), Keyable: true},
	}
}

func bumpSurveyCount(counts map[rule.Rule]measure.Count, f measure.Finding) {
	count := counts[f.Rule]
	count.Found++
	switch f.State {
	case measure.FindingIgnored:
		count.Ignored++
	case measure.FindingBaselined:
		count.Baselined++
	default:
		count.Reported++
	}
	counts[f.Rule] = count
}
