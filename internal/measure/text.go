package measure

import (
	"fmt"
	"io"
	"path/filepath"
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
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)

	fmt.Fprintf(tw, "Package\t%s\n", p.Path)
	if len(p.Config) > 0 {
		fmt.Fprintf(tw, "Config\t%s\n", strings.Join(p.Config, " + "))
	}
	if p.AllCore {
		// Not a note in passing: the two rules this command reports have
		// nothing to check here, and a table of zeros would say the opposite.
		fmt.Fprint(tw, "Note\tevery file is in the core namespace, so nothing crosses and no name is asked for one\n")
	}
	fmt.Fprintln(tw)

	asked := p.Findings[rule.Qualify].Asked
	writeTextNamespaces(tw, p, asked)
	writeTextCrossings(tw, p)
	writeTextReach(tw, p)
	writeTextQualify(tw, p, asked)

	return tw.Flush()
}

func writeTextNamespaces(w io.Writer, p Package, asked bool) {
	fmt.Fprint(w, "Namespaces\tfiles\tdeclarations\tqualify targets\n")
	for _, ns := range p.Namespaces {
		files := make([]string, 0, len(ns.Files))
		for _, f := range ns.Files {
			files = append(files, filepath.Base(f))
		}
		fmt.Fprintf(w, "  %s\t%s\t%d\t%s\n",
			ns.Name, strings.Join(files, ", "), ns.Declarations,
			textCount(ns.QualifyTargets, asked && !ns.Core))
	}
	fmt.Fprintln(w)
}

func writeTextCrossings(w io.Writer, p Package) {
	crossings := p.Crossings()
	fmt.Fprint(w, "Crossings\tmutual\tdeclared\tbaselined\treported\treached\tuses\n")
	if len(crossings) == 0 {
		fmt.Fprint(w, "  none\t\t\t\t\t\t\n")
	}
	var ignored, unchecked int
	for _, c := range crossings {
		ignored += c.Ignored
		unchecked += c.Unchecked
		fmt.Fprintf(w, "  %s → %s\t%s\t%d\t%d\t%d\t%d of %d\t%d\n",
			c.From, c.To, textYes(c.Mutual),
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
		fmt.Fprintf(w, "  (%s)\t\t\t\t\t\t\n", strings.Join(notes, "; "))
	}
	fmt.Fprintln(w)
}

func writeTextReach(w io.Writer, p Package) {
	reached := p.MostReached(10)
	if len(reached) == 0 {
		return
	}
	fmt.Fprint(w, "Most-reached declarations\tnamespace\treached from\tstate\n")
	for _, r := range reached {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
			r.Declaration, r.Namespace, textPlural(r.From, "namespace"), r.State)
	}
	fmt.Fprintln(w)
}

func writeTextQualify(w io.Writer, p Package, asked bool) {
	if !asked {
		// The rule was not in force. Printing the ratios anyway would answer
		// a question nobody put, and printing zeros would answer it wrongly.
		fmt.Fprint(w, "Qualify\tnot asked: rules.naming.qualify does not apply to this package\n\n")
		return
	}
	fmt.Fprint(w, "Qualify\texempt\tbaselined\treported\tsaturation\n")
	for _, q := range p.QualifyRows() {
		if q.Core || q.Targets == 0 {
			// Asked nothing, rather than asked and satisfied. A row of zeros
			// would say the second.
			fmt.Fprintf(w, "  %s\t%d\t-\t-\t-\n", q.Namespace, q.Exempt)
			continue
		}
		fmt.Fprintf(w, "  %s\t%d\t%d\t%d\t%d of %d\n",
			q.Namespace, q.Exempt, q.Baselined, q.Reported, q.Saturation(), q.Targets)
	}
	fmt.Fprintln(w)
}

// textCount prints a number, or a dash where the question was not asked. The
// two are different answers and a zero can only say one of them.
func textCount(n int, asked bool) string {
	if !asked {
		return "-"
	}
	return fmt.Sprint(n)
}

func textYes(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

func textPlural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
