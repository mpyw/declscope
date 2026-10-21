package unkeyed

// An unkeyed literal lists every field in declaration order, so each element
// writes one field. None of them is named here.
func buildMake() Entry { return Entry{1, 2, "x"} }

// A keyed literal naming only exported fields crosses nothing.
func buildExported() Entry { return Entry{ID: 3} }

// The same for an instantiation of a generic struct.
func buildPair() Pair[string] { return Pair[string]{4, "y"} }

// Each literal below leaves out a pointer type, and still writes the fields.
func buildElided() int {
	slots := []*Slot{{1}}
	cells := [...]*Cell{{1}}
	keys := map[*Key]int{{1}: 0}
	values := map[int]*Value{0: {1}}
	deep := [][]*Deep{{{1}}}
	named := []*Named{{count: 1}}
	return len(slots) + len(cells) + len(keys) + len(values) + len(deep) + len(named)
}

// The pointer may be a defined type, or a type parameter whose constraint
// allows only pointers to one struct.
func buildElidedNamed() int {
	slots := []SlotPtr{{1}}
	return len(slots)
}

func buildElidedParam[P interface{ *Snap }]() int {
	snaps := []P{{1}}
	return len(snaps)
}

// A constraint written with a tilde, in an interface the constraint embeds.
func buildElidedTilde[P interface{ SnapPtrs }]() int {
	snaps := []P{{1}}
	return len(snaps)
}

var _ = buildMake
var _ = buildPair
var _ = buildExported
var _ = buildElided
var _ = buildElidedNamed
var _ = buildElidedParam[*Snap]
var _ = buildElidedTilde[*Tilde]
