package measure

import (
	"cmp"
	"slices"
)

// Reach is one declaration and how many namespaces reach it.
//
// A crossing table cannot show this: a declaration reached from four
// namespaces appears there as four separate rows, one per reaching side, and
// the thing worth seeing is that they all land on the same declaration. That
// shape — one type's members read from every corner of the package — is a type
// filed by concern rather than a boundary anyone drew.
type Reach struct {
	Declaration string
	Kind        string

	// Namespace is where the declaration is written.
	Namespace string

	// From is how many namespaces reach it.
	From int

	// Uses counts every reference site outside its own namespace.
	Uses int

	// State is what became of the crossing. A declaration has exactly one,
	// however many namespaces reach it, because the analyzer judges the
	// declaration and not each use.
	State EdgeState
}

// MostReached returns the declarations reached from the most namespaces, at
// most limit of them. A limit of zero returns all of them.
func (p Package) MostReached(limit int) []Reach {
	byDecl := map[string]*Reach{}
	order := make([]string, 0, len(p.Edges))
	for _, e := range p.Edges {
		if e.State == EdgeOpen {
			continue
		}
		key := e.To + "." + e.Declaration
		r, ok := byDecl[key]
		if !ok {
			r = &Reach{
				Declaration: e.Declaration,
				Kind:        e.Kind,
				Namespace:   e.To,
				State:       e.State,
			}
			byDecl[key] = r
			order = append(order, key)
		}
		r.From++
		r.Uses += e.Uses
	}

	out := make([]Reach, 0, len(order))
	for _, key := range order {
		out = append(out, *byDecl[key])
	}
	slices.SortFunc(out, func(a, b Reach) int {
		return cmp.Or(
			-cmp.Compare(a.From, b.From),
			-cmp.Compare(a.Uses, b.Uses),
			cmp.Compare(a.Namespace, b.Namespace),
			cmp.Compare(a.Declaration, b.Declaration),
		)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
