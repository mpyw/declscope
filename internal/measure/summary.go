package measure

import (
	"cmp"
	"slices"

	"github.com/mpyw/declscope/internal/rule"
)

// Summary is many packages folded into the one question a whole run answers:
// which package to open first.
//
// The unit is the package throughout. A namespace is implicitly qualified by
// its package, so two namespaces that share a name share nothing else, and
// adding their counts would invent a unit that does not exist. Where a
// namespace is named here it is named inside a row that names its package,
// which is what makes it unambiguous.
type Summary struct {
	Checks Checks

	// Totals is one tally per rule, across every package.
	Totals map[rule.Rule]Count

	// Rows are the packages, heaviest first.
	Rows []SummaryRow
}

// SummaryRow is one package, with what it would cost to open it.
type SummaryRow struct {
	Package string

	// Namespaces and CoreFiles describe the shape rather than the findings.
	Namespaces int
	CoreFiles  int

	// AllCore marks a package where the boundary and naming rules have
	// nothing to check. The surplus, directive and filter rules still run, so
	// this is not the same as a package nothing looks at.
	AllCore bool

	// Boundary and Qualify are the states of that rule's findings.
	BoundaryReported  int
	BoundaryBaselined int
	BoundaryDeclared  int

	QualifyAsked     bool
	QualifyReported  int
	QualifyBaselined int
	QualifyExempt    int

	// Largest is the heaviest crossing in this package, and whether there is
	// one. It names a namespace, which this row may do because the row names
	// the package.
	Largest    Crossing
	HasLargest bool

	// Worst is the namespace the naming rule is least satisfied by. It is a
	// named unit and its own ratio rather than a count of namespaces past
	// some threshold: a ratio carries its own scale, where a threshold hides
	// a cutoff that one ignore directive can flip.
	Worst    QualifyRow
	HasWorst bool
}

// Weight is what the rows are sorted by: everything undecided, whether it is
// visible or deferred. A baselined finding is not resolved, it is postponed,
// so a package that deferred everything ranks with one that decided nothing —
// which is the package most worth opening.
func (r SummaryRow) Weight() int {
	return r.BoundaryReported + r.BoundaryBaselined + r.QualifyReported + r.QualifyBaselined
}

// SummaryOf folds the packages. Checks is passed in rather than derived,
// because which config files applied and whether the code compiled are facts
// about the run, not about any package's syntax.
func SummaryOf(pkgs []Package, checks Checks) Summary {
	out := Summary{Checks: checks, Totals: map[rule.Rule]Count{}}

	for _, p := range pkgs {
		for r, count := range p.Findings {
			total := out.Totals[r]
			total.Found += count.Found
			total.Ignored += count.Ignored
			total.Baselined += count.Baselined
			total.Reported += count.Reported
			// Asked anywhere is asked: a rule in force in one package and
			// inert in another has still answered for the run, and reporting
			// the total as "not asked" would hide the packages it answered
			// for. Where it applies is a per-package question, which the rows
			// below answer.
			total.Asked = total.Asked || count.Asked
			total.Keyable = count.Keyable
			out.Totals[r] = total
		}
		out.Rows = append(out.Rows, summaryRowOf(p))
	}

	slices.SortFunc(out.Rows, func(a, b SummaryRow) int {
		return cmp.Or(
			-cmp.Compare(a.Weight(), b.Weight()),
			cmp.Compare(a.Package, b.Package),
		)
	})
	return out
}

func summaryRowOf(p Package) SummaryRow {
	row := SummaryRow{
		Package:    p.Path,
		Namespaces: len(p.Namespaces),
		AllCore:    p.AllCore,

		BoundaryReported:  p.Findings[rule.Boundary].Reported,
		BoundaryBaselined: p.Findings[rule.Boundary].Baselined,

		QualifyAsked:     p.Findings[rule.Qualify].Asked,
		QualifyReported:  p.Findings[rule.Qualify].Reported,
		QualifyBaselined: p.Findings[rule.Qualify].Baselined,
	}
	for _, ns := range p.Namespaces {
		if ns.Core {
			row.CoreFiles += len(ns.Files)
		}
	}
	// Per declaration, not per edge: one helper shared with three namespaces
	// is one decision somebody took, not three.
	declared := map[string]bool{}
	for _, e := range p.Edges {
		if e.State == EdgeDeclared {
			declared[e.To+"."+e.Declaration] = true
		}
	}
	row.BoundaryDeclared = len(declared)
	for _, n := range p.Names {
		if n.State == NameExempt {
			row.QualifyExempt++
		}
	}
	if crossings := p.Crossings(); len(crossings) > 0 {
		row.Largest, row.HasLargest = crossings[0], true
	}
	row.Worst, row.HasWorst = p.WorstQualified()
	return row
}
