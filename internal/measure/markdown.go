package measure

import (
	"fmt"
	"io"
	"strings"

	"github.com/mpyw/declscope/internal/rule"
)

// WriteMarkdown renders one package for pasting somewhere: an issue, a pull
// request, a README, an article.
//
// It carries the same numbers as the text form and adds the diagram, which is
// what the format is for. A picture of a package's crossings belongs where
// pictures render, and nowhere else: in a terminal it would be a second way of
// saying what the table already says.
func (p Package) WriteMarkdown(w io.Writer) error {
	fmt.Fprintf(w, "# %s\n\n", p.Path)
	if len(p.Config) > 0 {
		fmt.Fprintf(w, "Config: `%s`\n\n", strings.Join(p.Config, "` + `"))
	}
	if p.AllCore {
		fmt.Fprint(w, "> Every file is in the core namespace, so nothing crosses and no name is asked to carry one.\n\n")
	}

	asked := p.Findings[rule.Qualify].Asked

	fmt.Fprint(w, "## Namespaces\n\n")
	fmt.Fprint(w, "| namespace | files | declarations | qualify targets |\n|---|---|---:|---:|\n")
	for _, ns := range p.Namespaces {
		files := make([]string, 0, len(ns.Files))
		for _, f := range ns.Files {
			files = append(files, "`"+f+"`")
		}
		fmt.Fprintf(w, "| %s | %s | %d | %s |\n",
			ns.Name, strings.Join(files, ", "), ns.Declarations,
			cellCount(ns.QualifyTargets, asked && !ns.Core))
	}

	writeMarkdownCrossings(w, p)
	writeMarkdownReach(w, p)
	writeMarkdownQualify(w, p, asked)
	return nil
}

func writeMarkdownCrossings(w io.Writer, p Package) {
	crossings := p.Crossings()
	fmt.Fprint(w, "\n## Crossings\n\n")
	if len(crossings) == 0 {
		fmt.Fprint(w, "Nothing crosses a namespace in this package.\n")
		return
	}
	fmt.Fprint(w, "| crossing | mutual | declared | baselined | reported | reached | uses |\n|---|---|---:|---:|---:|---:|---:|\n")
	for _, c := range crossings {
		fmt.Fprintf(w, "| %s → %s | %s | %d | %d | %d | %d of %d | %d |\n",
			c.From, c.To, cellYes(c.Mutual), c.Declared, c.Baselined, c.Reported,
			c.Reached, c.Declarations, c.Uses)
	}
	if open := p.OpenCrossings(); open > 0 {
		fmt.Fprintf(w, "\n%s further: open, package-scoped by default rather than by decision, so left out of the table and of the diagram.\n",
			cellPlural(open, "crossing is", "crossings are"))
	}
	writeMarkdownMermaid(w, crossings)
}

// writeMarkdownMermaid draws the same edges the table lists.
//
// One arrow per ordered pair, so a mutual pair is two arrows, as it is two
// rows. The line style is decided by the worst state present on the edge, by a
// fixed precedence rather than by what happens to come first: an edge carrying
// both a reported and a declared crossing has to render the same way in every
// run, or the goldens move on their own.
func writeMarkdownMermaid(w io.Writer, crossings []Crossing) {
	ids := map[string]string{}
	for _, c := range crossings {
		for _, ns := range []string{c.From, c.To} {
			if _, ok := ids[ns]; !ok {
				ids[ns] = fmt.Sprintf("n%d", len(ids))
			}
		}
	}

	fmt.Fprint(w, "\n```mermaid\ngraph LR\n")
	// Nodes are declared in the order the sorted crossings first name them,
	// which is stable for the same package.
	declared := map[string]bool{}
	for _, c := range crossings {
		for _, ns := range []string{c.From, c.To} {
			if !declared[ns] {
				fmt.Fprintf(w, "  %s[\"%s\"]\n", ids[ns], ns)
				declared[ns] = true
			}
		}
	}
	for _, c := range crossings {
		fmt.Fprintf(w, "  %s %s|%d| %s\n", ids[c.From], markdownArrow(c), c.Reached, ids[c.To])
	}
	fmt.Fprint(w, "```\n")
}

// markdownArrow is the line style for one crossing: thick where something is
// unresolved, plain where it is deferred, dotted where it was settled one way
// or another — declared shared, silenced by a directive, or not checked at
// all.
func markdownArrow(c Crossing) string {
	switch {
	case c.Reported > 0:
		return "==>"
	case c.Baselined > 0:
		return "-->"
	default:
		return "-.->"
	}
}

func writeMarkdownReach(w io.Writer, p Package) {
	reached := p.MostReached(10)
	if len(reached) == 0 {
		return
	}
	fmt.Fprint(w, "\n## Most-reached declarations\n\n")
	fmt.Fprint(w, "| declaration | namespace | reached from | state |\n|---|---|---:|---|\n")
	for _, r := range reached {
		fmt.Fprintf(w, "| `%s` | %s | %s | %s |\n",
			r.Declaration, r.Namespace, cellPlural(r.From, "namespace", "namespaces"), r.State)
	}
}

func writeMarkdownQualify(w io.Writer, p Package, asked bool) {
	fmt.Fprint(w, "\n## Qualify\n\n")
	if !asked {
		fmt.Fprint(w, "Not asked: `rules.naming.qualify` does not apply to this package.\n")
		return
	}
	fmt.Fprint(w, "| namespace | exempt | baselined | reported | saturation |\n|---|---:|---:|---:|---:|\n")
	for _, q := range p.QualifyRows() {
		if q.Core || q.Targets == 0 {
			fmt.Fprintf(w, "| %s | %d | - | - | - |\n", q.Namespace, q.Exempt)
			continue
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d of %d |\n",
			q.Namespace, q.Exempt, q.Baselined, q.Reported, q.Saturation(), q.Targets)
	}
}

// WriteSummaryMarkdown renders a whole run. It carries no diagram: the unit
// here is the package, and there is no edge set at that level to draw.
func (s Summary) WriteSummaryMarkdown(w io.Writer) error {
	fmt.Fprint(w, "## Checks in force\n\n")
	fmt.Fprint(w, "| check | value | packages |\n|---|---|---:|\n")
	for _, c := range s.Checks.Configs {
		chain := "built-in defaults"
		if len(c.Chain) > 0 {
			chain = "`" + strings.Join(c.Chain, "` + `") + "`"
		}
		qualify := c.Qualify
		if c.Exported {
			qualify += ", exported"
		}
		fmt.Fprintf(w, "| config | %s | %d |\n", chain, c.Packages)
		fmt.Fprintf(w, "| rules | boundary %s, qualify %s, surplus %s | %d |\n",
			cellOnOff(c.Boundary), qualify, cellOnOff(c.Surplus), c.Packages)
	}
	for _, b := range s.Checks.Baselines {
		fmt.Fprintf(w, "| baseline | `%s`, %d entries | |\n", b.Path, b.Entries)
	}
	fmt.Fprintf(w, "| type check | %s | |\n", cellTypeCheck(s.Checks.TypeCheck))

	fmt.Fprint(w, "\n## Findings\n\n")
	fmt.Fprint(w, "| rule | found | ignored | baselined | reported |\n|---|---:|---:|---:|---:|\n")
	for _, r := range rule.All {
		count, ok := s.Totals[r]
		if !ok {
			continue
		}
		if !count.Asked {
			fmt.Fprintf(w, "| `%s` | - | - | - | - |\n", r)
			continue
		}
		fmt.Fprintf(w, "| `%s` | %d | %d | %s | %d |\n",
			r, count.Found, count.Ignored,
			cellKeyable(count.Baselined, count.Keyable), count.Reported)
	}

	fmt.Fprint(w, "\n## Packages — boundary\n\n")
	fmt.Fprint(w, "| package | reported | baselined | declared | largest crossing |\n|---|---:|---:|---:|---|\n")
	for _, r := range s.Rows {
		name := "`" + r.Package + "`"
		if r.AllCore {
			name += " (all core)"
		}
		largest := "-"
		if r.HasLargest {
			largest = fmt.Sprintf("%s → %s (%s)", r.Largest.From, r.Largest.To,
				cellPlural(r.Largest.Reached, "declaration", "declarations"))
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %s |\n",
			name, r.BoundaryReported, r.BoundaryBaselined, r.BoundaryDeclared, largest)
	}

	fmt.Fprint(w, "\n## Packages — qualify\n\n")
	fmt.Fprint(w, "| package | reported | baselined | exempt | worst namespace |\n|---|---:|---:|---:|---|\n")
	for _, r := range s.Rows {
		if !r.QualifyAsked {
			fmt.Fprintf(w, "| `%s` | - | - | - | - |\n", r.Package)
			continue
		}
		worst := "-"
		if r.HasWorst {
			worst = fmt.Sprintf("%s %d of %d", r.Worst.Namespace, r.Worst.Saturation(), r.Worst.Targets)
		}
		fmt.Fprintf(w, "| `%s` | %d | %d | %d | %s |\n",
			r.Package, r.QualifyReported, r.QualifyBaselined, r.QualifyExempt, worst)
	}
	return nil
}
