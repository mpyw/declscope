package measure

// QualifyRow is one namespace's standing with the naming rule.
type QualifyRow struct {
	Namespace string

	// Core marks the namespace the rule never asks anything of, its prefix
	// being empty. Its counts are not zero; they do not exist.
	Core bool

	// Exempt counts declarations the rule reached and a directive excused.
	Exempt int

	Baselined int
	Reported  int

	// Targets is how many declarations the rule examines here, and the
	// denominator of Saturation. It is zero wherever the rule is not in
	// force, which is not the same as every declaration passing.
	Targets int
}

// Saturation is the share of this namespace's targets that fail, as a
// numerator over Targets.
//
// A baselined finding counts toward it. The baseline defers a decision rather
// than settling one, so a namespace whose every name is baselined is exactly
// as wrong as one whose every name is reported — and a regeneration would move
// it from one column to the other without a line of code changing.
//
// What a saturation means is not decided here. Near the top, the namespace
// name is usually what is wrong; in the middle, the file holds several
// concerns; near the bottom, the one declaration does. Which of those applies
// is a judgment, and this package reports the ratio.
func (q QualifyRow) Saturation() int { return q.Baselined + q.Reported }

// qualifyRows folds the naming findings per namespace, worst first, and lists
// every namespace, including those with nothing against them: a namespace the
// rule is satisfied by is the evidence that it was asked at all.
//
//declscope:package // markdown.go renders the rows
func (p Package) qualifyRows() []QualifyRow {
	byNamespace := map[string]*QualifyRow{}
	out := make([]QualifyRow, 0, len(p.Namespaces))
	for _, ns := range p.Namespaces {
		byNamespace[ns.Name] = &QualifyRow{
			Namespace: ns.Name,
			Core:      ns.Core,
			Targets:   ns.QualifyTargets,
		}
	}
	for _, n := range p.Names {
		row, ok := byNamespace[n.Namespace]
		if !ok {
			continue
		}
		switch n.State {
		case NameExempt:
			row.Exempt++
		case NameBaselined:
			row.Baselined++
		default:
			row.Reported++
		}
	}
	// Namespaces are already sorted, and this order is theirs: a fold that
	// reordered them would make two tables of the same package disagree about
	// which namespace comes first.
	for _, ns := range p.Namespaces {
		out = append(out, *byNamespace[ns.Name])
	}
	return out
}

// worstQualified is the namespace holding the most of a package's naming work,
// with its ratio beside it, and whether there is one at all.
//
// The count leads and the ratio breaks ties, because the other way round sends
// the reader to the smallest file in the package: a namespace failing two of
// two is fully saturated and two declarations of work, where one failing
// fifty-six of fifty-seven is the same shape and the whole job. Both are
// reported, so the ratio still says whether renaming the file would clear it.
//
// Naming one namespace is what a package-level report does instead of counting
// namespaces past a threshold, which would hide a cutoff that one ignore
// directive can flip.
//
//declscope:package // summary.go names it for each package
func (p Package) worstQualified() (QualifyRow, bool) {
	var worst QualifyRow
	found := false
	for _, row := range p.qualifyRows() {
		if row.Core || row.Targets == 0 || row.Saturation() == 0 {
			continue
		}
		switch {
		case !found,
			row.Saturation() > worst.Saturation(),
			row.Saturation() == worst.Saturation() && row.Saturation()*worst.Targets > worst.Saturation()*row.Targets:
			worst, found = row, true
		}
	}
	return worst, found
}
