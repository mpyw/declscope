package unkeyed

// An unkeyed literal lists every field in declaration order, so each element
// writes one field. None of them is named here.
func buildMake() Entry { return Entry{1, 2, "x"} }

// A keyed literal naming only exported fields crosses nothing.
func buildExported() Entry { return Entry{ID: 3} }

// The same for an instantiation of a generic struct.
func buildPair() Pair[string] { return Pair[string]{4, "y"} }

var _ = buildMake
var _ = buildPair
var _ = buildExported
