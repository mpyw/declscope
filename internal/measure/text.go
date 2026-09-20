package measure

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mpyw/declscope/internal/rule"
)

// text.go renders both reports for a terminal, as json.go and markdown.go
// render both: one file per format, so that a change to how a dash or a ratio
// is spelled lands in one place. It also keeps the method a plain WriteText on
// each type, where a file per type and format would have spelled the type into
// the name of every method on it.

// WriteText renders one package for a terminal: what it is, what its
// namespaces are, what reaches across them, and where the naming rule stands.
//
// Every number here is a fold of the model, and the model is a fold of what
// the analyzer found. Nothing is decided at this level, including what a
// saturation or a mutual crossing means; the tables carry the facts and the
// skill carries the reading.
func (p Package) WriteText(w io.Writer) error {
	out := newSink(tabwriter.NewWriter(w, 0, 0, 3, ' ', 0))

	out.printf("Package\t%s\n", p.Path)
	if len(p.Config) > 0 {
		out.printf("Config\t%s\n", strings.Join(p.Config, " + "))
	}
	if p.AllCore {
		// Not a note in passing: the two rules this command reports have
		// nothing to check here, and a table of zeros would say the opposite.
		out.print("Note\tevery file is in the core namespace, so nothing crosses and no name is asked for one\n")
	}
	out.print("\n")

	asked := p.Findings[rule.Qualify].Asked
	writeTextNamespaces(out, p, asked)
	writeTextCrossings(out, p, p.Findings[rule.Boundary].Asked)
	writeTextReach(out, p)
	writeTextQualify(out, p, asked)

	return out.flush()
}

func writeTextNamespaces(out *sink, p Package, asked bool) {
	out.print("Namespaces\tfiles\tdeclarations\tqualify targets\n")
	for _, ns := range p.Namespaces {
		out.printf("  %s\t%s\t%d\t%s\n",
			ns.Name, strings.Join(ns.Files, ", "), ns.Declarations,
			cellCount(ns.QualifyTargets, asked && !ns.Core))
	}
	out.print("\n")
}

func writeTextCrossings(out *sink, p Package, asked bool) {
	crossings := p.Crossings()
	out.print("Crossings\tmutual\tdeclared\tbaselined\treported\treached\tuses\n")
	if len(crossings) == 0 {
		out.print("  none\t\t\t\t\t\t\n")
	}
	var ignored, unchecked int
	for _, c := range crossings {
		ignored += c.Ignored
		unchecked += c.Unchecked
		out.printf("  %s → %s\t%s\t%s\t%s\t%s\t%d of %d\t%d\n",
			c.From, c.To, cellYes(c.Mutual),
			cellCount(c.Declared, asked), cellCount(c.Baselined, asked), cellCount(c.Reported, asked),
			c.Reached, c.Declarations, c.Uses)
	}

	// The three states the table has no column for. Silenced and unchecked
	// crossings are rare enough not to earn one, and open ones are left out of
	// the table on purpose — but a reader who is told nothing about them would
	// read the columns above as the whole of what crosses.
	var notes []string
	if ignored > 0 {
		notes = append(notes, fmt.Sprintf("%d silenced by a directive", ignored))
	}
	if unchecked > 0 {
		notes = append(notes, fmt.Sprintf("%d not checked, rules.allowBoundary is on", unchecked))
	}
	if open := p.OpenCrossings(); open > 0 {
		notes = append(notes, fmt.Sprintf("%d open, package-scoped by default rather than by decision", open))
	}
	if len(notes) > 0 {
		// No tabs: a line inside the block would set the width of the first
		// column, and this one is a sentence.
		out.printf("  (%s)\n", strings.Join(notes, "; "))
	}
	out.print("\n")
}

func writeTextReach(out *sink, p Package) {
	reached := p.MostReached(10)
	if len(reached) == 0 {
		return
	}
	out.print("Most-reached declarations\tnamespace\treached from\tstate\n")
	for _, r := range reached {
		out.printf("  %s\t%s\t%s\t%s\n",
			r.Declaration, r.Namespace, cellPlural(r.From, "namespace", "namespaces"), r.State)
	}
	out.print("\n")
}

func writeTextQualify(out *sink, p Package, asked bool) {
	if !asked {
		// The rule was not in force. Printing the ratios anyway would answer
		// a question nobody put, and printing zeros would answer it wrongly.
		out.print("Qualify\tnot asked: rules.naming.qualify does not apply to this package\n\n")
		return
	}
	out.print("Qualify\texempt\tbaselined\treported\tsaturation\n")
	for _, q := range p.QualifyRows() {
		if q.Core || q.Targets == 0 {
			// Asked nothing, rather than asked and satisfied. A row of zeros
			// would say the second.
			out.printf("  %s\t%d\t-\t-\t-\n", q.Namespace, q.Exempt)
			continue
		}
		out.printf("  %s\t%d\t%d\t%d\t%d of %d\n",
			q.Namespace, q.Exempt, q.Baselined, q.Reported, q.Saturation(), q.Targets)
	}
	out.print("\n")
}

// WriteSummaryText renders a whole run: what was checked, what was found, and
// which package to open first.
func (s Summary) WriteText(w io.Writer) error {
	out := newSink(tabwriter.NewWriter(w, 0, 0, 3, ' ', 0))

	writeSummaryTextChecks(out, s.Checks)
	writeSummaryTextFindings(out, s.Totals)
	writeSummaryTextBoundary(out, s.Rows)
	writeSummaryTextQualify(out, s.Rows)

	return out.flush()
}

// writeSummaryTextChecks prints the state of the checks before any count,
// because a count means nothing until the reader knows the rule was in force,
// the code compiled, and what a baseline is absorbing.
func writeSummaryTextChecks(out *sink, checks Checks) {
	out.print("Checks in force\t\t\n")
	for _, c := range checks.Configs {
		chain := "built-in defaults"
		if len(c.Chain) > 0 {
			chain = strings.Join(c.Chain, " + ")
		}
		out.printf("  config\t%s\t%s\n", chain, cellPackages(c.Packages))
	}
	if len(checks.Configs) > 0 {
		// Every config in one run switches the same rules on or off for the
		// packages it governs, so the rule lines are per config rather than
		// one set for the whole run.
		for _, c := range checks.Configs {
			qualify := c.Qualify
			if c.Exported {
				qualify += ", exported"
			}
			out.printf("  rules\tboundary %s, qualify %s, surplus %s\t%s\n",
				cellOnOff(c.Boundary), qualify, cellOnOff(c.Surplus),
				cellPackages(c.Packages))
		}
	}
	for _, b := range checks.Baselines {
		out.printf("  baseline\t%s\t%s\n", b.Path, cellPlural(b.Entries, "entry", "entries"))
	}
	out.printf("  type check\t%s\t\n", cellTypeCheck(checks.TypeCheck))
	out.print("\n")
}

func writeSummaryTextFindings(out *sink, totals map[rule.Rule]Count) {
	out.print("Findings\tfound\tignored\tbaselined\treported\n")
	var sum Count
	for _, r := range rule.All {
		count, ok := totals[r]
		if !ok {
			continue
		}
		if !count.Asked {
			// Switched off everywhere it could have applied. Four zeros would
			// read as a clean run rather than as a question nobody put.
			out.printf("  %s\t-\t-\t-\t-\n", r)
			continue
		}
		out.printf("  %s\t%d\t%d\t%s\t%d\n",
			r, count.Found, count.Ignored,
			cellKeyable(count.Baselined, count.Keyable), count.Reported)
		sum.Found += count.Found
		sum.Ignored += count.Ignored
		sum.Baselined += count.Baselined
		sum.Reported += count.Reported
	}
	out.printf("  total\t%d\t%d\t%d\t%d\n", sum.Found, sum.Ignored, sum.Baselined, sum.Reported)
	out.print("\n")
}

func writeSummaryTextBoundary(out *sink, rows []SummaryRow) {
	out.print("Packages — boundary\treported\tbaselined\tdeclared\tlargest crossing\n")
	for _, r := range rows {
		name := r.Package
		if r.AllCore {
			name += " [all core]"
		}
		largest := "-"
		if r.HasLargest {
			largest = fmt.Sprintf("%s → %s (%s)",
				r.Largest.From, r.Largest.To,
				cellPlural(r.Largest.Reached, "declaration", "declarations"))
		}
		out.printf("  %s\t%d\t%d\t%d\t%s\n",
			name, r.BoundaryReported, r.BoundaryBaselined, r.BoundaryDeclared, largest)
	}
	out.print("\n")
}

func writeSummaryTextQualify(out *sink, rows []SummaryRow) {
	out.print("Packages — qualify\treported\tbaselined\texempt\tworst namespace\n")
	for _, r := range rows {
		if !r.QualifyAsked {
			out.printf("  %s\t-\t-\t-\t-\n", r.Package)
			continue
		}
		worst := "-"
		if r.HasWorst {
			worst = fmt.Sprintf("%s %d of %d", r.Worst.Namespace, r.Worst.Saturation(), r.Worst.Targets)
		}
		out.printf("  %s\t%d\t%d\t%d\t%s\n",
			r.Package, r.QualifyReported, r.QualifyBaselined, r.QualifyExempt, worst)
	}
	out.print("\n")
}
