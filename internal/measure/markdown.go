package measure

import (
	"fmt"
	"io"
	"strings"

	"github.com/mpyw/declscope/internal/rule"
)

// writeMarkdown renders one package for pasting somewhere: an issue, a pull
// request, a README, an article.
//
// It carries the same numbers as the text form and adds the diagram, which is
// what the format is for. A picture of a package's crossings belongs where
// pictures render, and nowhere else: in a terminal it would be a second way of
// saying what the table already says.
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

	out.print("## Namespaces\n\n")
	out.print("| namespace | files | declarations | qualify targets |\n|---|---|---:|---:|\n")
	for _, ns := range p.Namespaces {
		files := make([]string, 0, len(ns.Files))
		for _, f := range ns.Files {
			files = append(files, "`"+f+"`")
		}
		out.printf("| %s | %s | %d | %s |\n",
			ns.Name, strings.Join(files, ", "), ns.Declarations,
			cellCount(ns.QualifyTargets, asked && !ns.Core))
	}

	writeMarkdownCrossings(out, p, p.Findings[rule.Boundary].Asked)
	writeMarkdownReach(out, p)
	writeMarkdownQualify(out, p, asked)
	return out.flush()
}

func writeMarkdownCrossings(out *sink, p Package, asked bool) {
	crossings := p.Crossings()
	out.print("\n## Crossings\n\n")
	if len(crossings) == 0 {
		out.print("Nothing crosses a namespace in this package.\n")
		return
	}
	out.print("| crossing | mutual | declared | baselined | reported | reached | uses |\n|---|---|---:|---:|---:|---:|---:|\n")
	for _, c := range crossings {
		out.printf("| %s → %s | %s | %s | %s | %s | %d of %d | %d |\n",
			c.From, c.To, cellYes(c.Mutual),
			cellCount(c.Declared, asked), cellCount(c.Baselined, asked), cellCount(c.Reported, asked),
			c.Reached, c.Declarations, c.Uses)
	}
	if open := p.DeclarationsCrossing(EdgeOpen); open > 0 {
		out.printf("\n%s further: open, package-scoped by default rather than by decision, so left out of the table and of the diagram.\n",
			cellPlural(open, "declaration is", "declarations are"))
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
	out.print("\n## Most-reached declarations\n\n")
	out.print("| declaration | namespace | reached from | state |\n|---|---|---:|---|\n")
	for _, r := range reached {
		out.printf("| `%s` | %s | %s | %s |\n",
			r.Declaration, r.Namespace, cellPlural(r.From, "namespace", "namespaces"), r.State)
	}
}

func writeMarkdownQualify(out *sink, p Package, asked bool) {
	out.print("\n## Qualify\n\n")
	if !asked {
		out.print("Not asked: `rules.naming.qualify` does not apply to this package.\n")
		return
	}
	out.print("| namespace | exempt | baselined | reported | saturation |\n|---|---:|---:|---:|---:|\n")
	for _, q := range p.QualifyRows() {
		if q.Core || q.Targets == 0 {
			// Nothing was examined here, so nothing was excused either:
			// printing 0 would answer a question that was not put.
			out.print("| " + q.Namespace + " | - | - | - | - |\n")
			continue
		}
		out.printf("| %s | %d | %d | %d | %d of %d |\n",
			q.Namespace, q.Exempt, q.Baselined, q.Reported, q.Saturation(), q.Targets)
	}
}

// writeMarkdown renders a whole run. It carries no diagram: the unit
// here is the package, and there is no edge set at that level to draw.
//
//declscope:package // format.go dispatches to it
func (s Summary) writeMarkdown(w io.Writer) error {
	out := newSink(w)
	out.print("## Checks in force\n\n")
	out.print("| check | value | packages |\n|---|---|---:|\n")
	for _, c := range s.Checks.Configs {
		chain := "built-in defaults"
		if len(c.Chain) > 0 {
			chain = "`" + strings.Join(c.Chain, "` + `") + "`"
		}
		qualify := c.Qualify
		if c.Exported {
			qualify += ", exported"
		}
		out.printf("| config | %s | %d |\n", chain, c.Packages)
		out.printf("| rules | boundary %s, qualify %s, surplus %s | %d |\n",
			cellOnOff(c.Boundary), qualify, cellOnOff(c.Surplus), c.Packages)
	}
	for _, b := range s.Checks.Baselines {
		out.printf("| baseline | `%s`, %s | |\n", b.Path, cellPlural(b.Entries, "entry", "entries"))
	}
	out.printf("| type check | %s | |\n", cellTypeCheck(s.Checks.TypeCheck))

	out.print("\n## Findings\n\n")
	out.print("| rule | found | ignored | baselined | reported |\n|---|---:|---:|---:|---:|\n")
	for _, r := range rule.All {
		count, ok := s.Totals[r]
		if !ok {
			continue
		}
		if !count.Asked {
			out.printf("| `%s` | - | - | - | - |\n", r)
			continue
		}
		out.printf("| `%s` | %d | %d | %s | %d |\n",
			r, count.Found, count.Ignored,
			cellKeyable(count.Baselined, count.Keyable), count.Reported)
	}

	out.print("\n## Packages — boundary\n\n")
	out.print("| package | reported | baselined | declared | largest crossing |\n|---|---:|---:|---:|---|\n")
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
		out.printf("| %s | %d | %d | %d | %s |\n",
			name, r.BoundaryReported, r.BoundaryBaselined, r.BoundaryDeclared, largest)
	}

	out.print("\n## Packages — qualify\n\n")
	out.print("| package | reported | baselined | exempt | worst namespace |\n|---|---:|---:|---:|---|\n")
	for _, r := range s.Rows {
		if !r.QualifyAsked {
			out.printf("| `%s` | - | - | - | - |\n", r.Package)
			continue
		}
		worst := "-"
		if r.HasWorst {
			worst = fmt.Sprintf("%s %d of %d", r.Worst.Namespace, r.Worst.Saturation(), r.Worst.Targets)
		}
		out.printf("| `%s` | %d | %d | %d | %s |\n",
			r.Package, r.QualifyReported, r.QualifyBaselined, r.QualifyExempt, worst)
	}
	return out.flush()
}
