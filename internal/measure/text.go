package measure

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mpyw/declscope/internal/rule"
)

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
	writeTextCrossings(out, p)
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

func writeTextCrossings(out *sink, p Package) {
	crossings := p.Crossings()
	out.print("Crossings\tmutual\tdeclared\tbaselined\treported\treached\tuses\n")
	if len(crossings) == 0 {
		out.print("  none\t\t\t\t\t\t\n")
	}
	var ignored, unchecked int
	for _, c := range crossings {
		ignored += c.Ignored
		unchecked += c.Unchecked
		out.printf("  %s → %s\t%s\t%d\t%d\t%d\t%d of %d\t%d\n",
			c.From, c.To, cellYes(c.Mutual),
			c.Declared, c.Baselined, c.Reported,
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
