package measure

import (
	"cmp"
	"slices"
)

// Crossing is every declaration one namespace reaches in another, folded into
// one row.
//
// One row is one ordered pair. A mutual pair stays two rows, marked, because
// folding it into one would lose the count in each direction, and the two
// directions are rarely alike: the shape worth recognizing is A reaching most
// of B while B reaches almost nothing of A.
type Crossing struct {
	// From is the namespace doing the reaching, To the one being reached.
	From, To string

	// Mutual marks a pair whose opposite row also exists. Rows are sorted by
	// weight, so the opposite row is usually nowhere near this one; without
	// the mark the reader has to scan the table for it.
	Mutual bool

	// The declarations reached, by what became of each.
	Declared  int
	Baselined int
	Reported  int
	Ignored   int
	Unchecked int

	// Reached is how many of the reached namespace's declarations this row
	// touches, over Declarations, which is that namespace's total. The
	// denominator belongs to the reached side alone, so every row reaching the
	// same namespace divides by the same number.
	Reached      int
	Declarations int

	// Uses counts reference sites. It is not the sum of the state counts,
	// which are per declaration.
	Uses int
}

// Crossings folds the edge set into one row per ordered pair, heaviest first.
//
// Open edges are left out. They are declarations that are package-scoped
// because nothing says otherwise, so listing them would bury the crossings
// somebody decided on under the ones nobody was ever asked about. The count of
// them is reported separately.
func (p Package) Crossings() []Crossing {
	declarations := map[string]int{}
	for _, ns := range p.Namespaces {
		declarations[ns.Name] = ns.Declarations
	}

	byPair := map[[2]string]*Crossing{}
	for _, e := range p.Edges {
		if e.State == EdgeOpen {
			continue
		}
		key := [2]string{e.From, e.To}
		c, ok := byPair[key]
		if !ok {
			c = &Crossing{From: e.From, To: e.To, Declarations: declarations[e.To]}
			byPair[key] = c
		}
		c.Reached++
		c.Uses += e.Uses
		switch e.State {
		case EdgeDeclared:
			c.Declared++
		case EdgeBaselined:
			c.Baselined++
		case EdgeReported:
			c.Reported++
		case EdgeIgnored:
			c.Ignored++
		case EdgeUnchecked:
			c.Unchecked++
		}
	}

	out := make([]Crossing, 0, len(byPair))
	for key, c := range byPair {
		_, c.Mutual = byPair[[2]string{key[1], key[0]}]
		out = append(out, *c)
	}
	slices.SortFunc(out, func(a, b Crossing) int {
		return cmp.Or(
			// Heaviest first, by declarations reached rather than by uses: the
			// decision this table feeds is whether one namespace holds the
			// working parts of another, and breadth answers that where depth
			// only says a helper is popular.
			-cmp.Compare(a.Reached, b.Reached),
			-cmp.Compare(a.Uses, b.Uses),
			cmp.Compare(a.From, b.From),
			cmp.Compare(a.To, b.To),
		)
	})
	return out
}

// OpenCrossings counts the declarations reached across a namespace that are
// package-scoped by default rather than by decision.
func (p Package) OpenCrossings() int {
	n := 0
	for _, e := range p.Edges {
		if e.State == EdgeOpen {
			n++
		}
	}
	return n
}
