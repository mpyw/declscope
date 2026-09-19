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
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)

	writeSummaryTextChecks(tw, s.Checks)
	writeSummaryTextFindings(tw, s.Totals)
	writeSummaryTextBoundary(tw, s.Rows)
	writeSummaryTextQualify(tw, s.Rows)

	return tw.Flush()
}

// writeSummaryTextChecks prints the state of the checks before any count,
// because a count means nothing until the reader knows the rule was in force,
// the code compiled, and what a baseline is absorbing.
func writeSummaryTextChecks(w io.Writer, checks Checks) {
	fmt.Fprint(w, "Checks in force\t\t\n")
	for _, c := range checks.Configs {
		chain := "built-in defaults"
		if len(c.Chain) > 0 {
			chain = strings.Join(c.Chain, " + ")
		}
		fmt.Fprintf(w, "  config\t%s\t%s\n", chain, summaryTextPackages(c.Packages))
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
			fmt.Fprintf(w, "  rules\tboundary %s, qualify %s, surplus %s\t%s\n",
				summaryTextOnOff(c.Boundary), qualify, summaryTextOnOff(c.Surplus),
				summaryTextPackages(c.Packages))
		}
	}
	for _, b := range checks.Baselines {
		fmt.Fprintf(w, "  baseline\t%s\t%s\n", b.Path, summaryTextPlural(b.Entries, "entry", "entries"))
	}
	fmt.Fprintf(w, "  type check\t%s\t\n", summaryTextTypeCheck(checks.TypeCheck))
	fmt.Fprintln(w)
}

func writeSummaryTextFindings(w io.Writer, totals map[rule.Rule]Count) {
	fmt.Fprint(w, "Findings\tfound\tignored\tbaselined\treported\n")
	var sum Count
	for _, r := range rule.All {
		count, ok := totals[r]
		if !ok {
			continue
		}
		if !count.Asked {
			// Switched off everywhere it could have applied. Four zeros would
			// read as a clean run rather than as a question nobody put.
			fmt.Fprintf(w, "  %s\t-\t-\t-\t-\n", r)
			continue
		}
		fmt.Fprintf(w, "  %s\t%d\t%d\t%s\t%d\n",
			r, count.Found, count.Ignored,
			summaryTextKeyable(count.Baselined, count.Keyable), count.Reported)
		sum.Found += count.Found
		sum.Ignored += count.Ignored
		sum.Baselined += count.Baselined
		sum.Reported += count.Reported
	}
	fmt.Fprintf(w, "  total\t%d\t%d\t%d\t%d\n", sum.Found, sum.Ignored, sum.Baselined, sum.Reported)
	fmt.Fprintln(w)
}

func writeSummaryTextBoundary(w io.Writer, rows []SummaryRow) {
	fmt.Fprint(w, "Packages — boundary\treported\tbaselined\tdeclared\tlargest crossing\n")
	for _, r := range rows {
		name := r.Package
		if r.AllCore {
			name += " [all core]"
		}
		largest := "-"
		if r.HasLargest {
			largest = fmt.Sprintf("%s → %s (%s)",
				r.Largest.From, r.Largest.To,
				summaryTextPlural(r.Largest.Reached, "declaration", "declarations"))
		}
		fmt.Fprintf(w, "  %s\t%d\t%d\t%d\t%s\n",
			name, r.BoundaryReported, r.BoundaryBaselined, r.BoundaryDeclared, largest)
	}
	fmt.Fprintln(w)
}

func writeSummaryTextQualify(w io.Writer, rows []SummaryRow) {
	fmt.Fprint(w, "Packages — qualify\treported\tbaselined\texempt\tworst namespace\n")
	for _, r := range rows {
		if !r.QualifyAsked {
			fmt.Fprintf(w, "  %s\t-\t-\t-\t-\n", r.Package)
			continue
		}
		worst := "-"
		if r.HasWorst {
			worst = fmt.Sprintf("%s %d of %d", r.Worst.Namespace, r.Worst.Saturation(), r.Worst.Targets)
		}
		fmt.Fprintf(w, "  %s\t%d\t%d\t%d\t%s\n",
			r.Package, r.QualifyReported, r.QualifyBaselined, r.QualifyExempt, worst)
	}
	fmt.Fprintln(w)
}

// summaryTextKeyable prints a dash where no baseline could ever suppress the
// rule, which is the directive rule and the filter rule. A zero there would
// read as "suppressible, and none suppressed".
func summaryTextKeyable(n int, keyable bool) string {
	if !keyable {
		return "-"
	}
	return fmt.Sprint(n)
}

func summaryTextTypeCheck(t TypeCheck) string {
	return fmt.Sprintf("%s ok, %d failed", summaryTextPackages(t.Packages-len(t.Failed)), len(t.Failed))
}

func summaryTextOnOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func summaryTextPackages(n int) string {
	return summaryTextPlural(n, "package", "packages")
}

func summaryTextPlural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
