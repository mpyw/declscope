// summarytext.go renders a Summary for a terminal. It is its own file rather
// than a second half of text.go because the two answer different questions —
// which package to open, and what shape one package is in — and a reader
// looking for one should not have to read past the other.
//
//declscope:namespace summarytext

package measure

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mpyw/declscope/internal/rule"
)

// WriteSummaryText renders a whole run: what was checked, what was found, and
// which package to open first.
func (s Summary) WriteSummaryText(w io.Writer) error {
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
