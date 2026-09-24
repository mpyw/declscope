package measure

import (
	"fmt"
	"io"
	"strings"

	"github.com/mpyw/declscope/internal/rule"
)

// writeMarkdown renders one package as padded Markdown: readable in a terminal
// and ready to paste into an issue, a pull request or a README.
//
//declscope:package // format.go dispatches to it
func (p Package) writeMarkdown(w io.Writer) error {
	out := newSink(w)
	out.printf("# %s\n\n", p.Path)
	if len(p.Config) > 0 {
		out.printf("Config: `%s`\n\n", strings.Join(p.Config, "` + `"))
	}
	if len(p.Namespaces) == 0 {
		out.print("> No file of this package was read — every one is generated, or the filter removed them — so nothing here was checked.\n\n")
	}
	if p.AllCore {
		out.print("> Every file is in the core namespace, so nothing crosses and no name is asked to carry one.\n\n")
	}

	asked := p.Findings[rule.Qualify].Asked

	rows := [][]string{{"namespace", "files", "declarations", "qualify targets"}}
	for _, ns := range p.Namespaces {
		rows = append(rows, []string{
			ns.Name, "`" + strings.Join(ns.Files, "`, `") + "`",
			fmt.Sprint(ns.Declarations),
			cellCount(ns.QualifyTargets, asked && !ns.Core),
		})
	}
	writeMarkdownTable(out, "Namespaces", rows)

	writeMarkdownCrossings(out, p, p.Findings[rule.Boundary].Asked)
	writeMarkdownReach(out, p)
	writeMarkdownQualify(out, p, asked)
	return out.flush()
}

func writeMarkdownCrossings(out *sink, p Package, asked bool) {
	crossings := p.Crossings()
	if len(crossings) == 0 {
		open := p.DeclarationsCrossing(EdgeOpen)
		if open == 0 {
			out.print("## Crossings\n\nNothing crosses a namespace in this package.\n\n")
		} else {
			out.printf("## Crossings\n\nOpen crossings only: %s package-scoped by default rather than by decision, so there is nothing to put in the table or diagram.\n\n",
				cellPlural(open, "declaration is", "declarations are"))
		}
		return
	}
	rows := [][]string{{"crossing", "mutual", "declared", "baselined", "reported", "clears", "reached", "uses"}}
	for _, c := range crossings {
		rows = append(rows, []string{
			c.From + " → " + c.To, cellYes(c.Mutual),
			cellCount(c.Declared, asked), cellCount(c.Baselined, asked), cellCount(c.Reported, asked),
			cellCount(c.Clears, asked),
			fmt.Sprintf("%d of %d", c.Reached, c.Declarations),
			fmt.Sprint(c.Uses),
		})
	}
	writeMarkdownTable(out, "Crossings", rows)
	if open := p.DeclarationsCrossing(EdgeOpen); open > 0 {
		out.printf("%s further: open, package-scoped by default rather than by decision, so left out of the table and of the diagram.\n",
			cellPlural(open, "declaration is", "declarations are"))
	}
	out.print("\n")
	if asked {
		out.printf("Declarations crossed: %d reported, %d baselined, %d declared. The columns above count within a pair, so they sum to more: a declaration reached from two namespaces is two rows and one finding.\n\n",
			p.DeclarationsCrossing(EdgeReported),
			p.DeclarationsCrossing(EdgeBaselined),
			p.DeclarationsCrossing(EdgeDeclared))
	}
	writeMarkdownMermaid(out, crossings)
}

// writeMarkdownMermaid draws the same edges the table lists.
//
// One arrow per ordered pair, so a mutual pair is two arrows, as it is two
// rows. The line style is decided by the worst state present on the edge, by a
// fixed precedence rather than by what happens to come first: an edge carrying
// both a reported and a declared crossing has to render the same way in every
// run, or the goldens move on their own.
func writeMarkdownMermaid(out *sink, crossings []Crossing) {
	ids := map[string]string{}
	for _, c := range crossings {
		for _, ns := range []string{c.From, c.To} {
			if _, ok := ids[ns]; !ok {
				ids[ns] = fmt.Sprintf("n%d", len(ids))
			}
		}
	}

	out.print("\n```mermaid\ngraph LR\n")
	// Nodes are declared in the order the sorted crossings first name them,
	// which is stable for the same package.
	declared := map[string]bool{}
	for _, c := range crossings {
		for _, ns := range []string{c.From, c.To} {
			if !declared[ns] {
				out.printf("  %s[\"%s\"]\n", ids[ns], ns)
				declared[ns] = true
			}
		}
	}
	for _, c := range crossings {
		out.printf("  %s %s|%d| %s\n", ids[c.From], markdownArrow(c), c.Reached, ids[c.To])
	}
	out.print("```\n")
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

func writeMarkdownReach(out *sink, p Package) {
	reached := p.MostReached(10)
	if len(reached) == 0 {
		return
	}
	rows := [][]string{{"declaration", "namespace", "reached from", "state"}}
	for _, r := range reached {
		rows = append(rows, []string{
			"`" + r.Declaration + "`", r.Namespace,
			cellPlural(r.From, "namespace", "namespaces"), string(r.State),
		})
	}
	writeMarkdownTable(out, "Most-reached declarations", rows)
}

func writeMarkdownQualify(out *sink, p Package, asked bool) {
	if !asked {
		out.print("## Qualify\n\nNot asked: `rules.naming.qualify` does not apply to this package.\n\n")
		return
	}
	rows := [][]string{{"namespace", "exempt", "baselined", "reported", "saturation"}}
	for _, q := range p.QualifyRows() {
		if q.Core || q.Targets == 0 {
			// Nothing was examined here, so nothing was excused either:
			// printing 0 would answer a question that was not put.
			rows = append(rows, []string{q.Namespace, "-", "-", "-", "-"})
			continue
		}
		rows = append(rows, []string{
			q.Namespace, fmt.Sprint(q.Exempt), fmt.Sprint(q.Baselined), fmt.Sprint(q.Reported),
			fmt.Sprintf("%d of %d", q.Saturation(), q.Targets),
		})
	}
	writeMarkdownTable(out, "Qualify", rows)
}

// writeMarkdown renders a whole run. It carries no diagram: the unit
// here is the package, and there is no edge set at that level to draw.
//
//declscope:package // format.go dispatches to it
func (s Summary) writeMarkdown(w io.Writer) error {
	out := newSink(w)
	rows := [][]string{{"check", "value", "packages"}}
	for _, c := range s.Checks.Configs {
		chain := "built-in defaults"
		if len(c.Chain) > 0 {
			chain = "`" + strings.Join(c.Chain, "` + `") + "`"
		}
		qualify := c.Qualify
		if c.Exported {
			qualify += ", exported"
		}
		rows = append(rows,
			[]string{"config", chain, fmt.Sprint(c.Packages)},
			[]string{"rules", fmt.Sprintf("boundary %s, qualify %s, surplus %s, unused %s",
				cellOnOff(c.Boundary), qualify, c.Surplus, c.Unused), fmt.Sprint(c.Packages)})
	}
	for _, b := range s.Checks.Baselines {
		rows = append(rows, []string{"baseline",
			fmt.Sprintf("`%s`, %s", b.Path, cellPlural(b.Entries, "entry", "entries")), ""})
	}
	rows = append(rows, []string{"type check", cellTypeCheck(s.Checks.TypeCheck), ""})
	writeMarkdownTable(out, "Checks in force", rows)

	findings := [][]string{{"rule", "found", "ignored", "baselined", "reported"}}
	for _, r := range rule.All {
		count, ok := s.Totals[r]
		if !ok {
			continue
		}
		if !count.Asked {
			findings = append(findings, []string{"`" + string(r) + "`", "-", "-", "-", "-"})
			continue
		}
		findings = append(findings, []string{
			"`" + string(r) + "`", fmt.Sprint(count.Found), fmt.Sprint(count.Ignored),
			cellKeyable(count.Baselined, count.Keyable), fmt.Sprint(count.Reported),
		})
	}
	writeMarkdownTable(out, "Findings", findings)

	boundary := [][]string{{"package", "reported", "baselined", "declared", "largest crossing"}}
	for _, r := range s.Rows {
		name := "`" + r.Package + "`"
		switch {
		case r.Namespaces == 0:
			name += " [nothing read]"
		case r.AllCore:
			name += " [all core]"
		}
		largest := "-"
		if r.HasLargest {
			largest = fmt.Sprintf("%s → %s (%s)", r.Largest.From, r.Largest.To,
				cellPlural(r.Largest.Reached, "declaration", "declarations"))
		}
		if !r.BoundaryAsked {
			boundary = append(boundary, []string{name, "-", "-", "-", largest})
			continue
		}
		boundary = append(boundary, []string{
			name, fmt.Sprint(r.BoundaryReported), fmt.Sprint(r.BoundaryBaselined),
			fmt.Sprint(r.BoundaryDeclared), largest,
		})
	}
	writeMarkdownTable(out, "Packages — boundary", boundary)

	qualify := [][]string{{"package", "reported", "baselined", "exempt", "worst namespace"}}
	for _, r := range s.Rows {
		if !r.QualifyAsked {
			qualify = append(qualify, []string{"`" + r.Package + "`", "-", "-", "-", "-"})
			continue
		}
		worst := "-"
		if r.HasWorst {
			worst = fmt.Sprintf("%s %d of %d", r.Worst.Namespace, r.Worst.Saturation(), r.Worst.Targets)
		}
		qualify = append(qualify, []string{
			"`" + r.Package + "`", fmt.Sprint(r.QualifyReported), fmt.Sprint(r.QualifyBaselined),
			fmt.Sprint(r.QualifyExempt), worst,
		})
	}
	writeMarkdownTable(out, "Packages — qualify", qualify)
	return out.flush()
}

// writeMarkdownTable prints one padded table under its heading. Every table in
// this file goes through it, so a column added anywhere lines up everywhere.
func writeMarkdownTable(out *sink, heading string, rows [][]string) {
	out.printf("## %s\n\n", heading)
	for _, line := range cellTable(rows) {
		out.print(line + "\n")
	}
	out.print("\n")
}
