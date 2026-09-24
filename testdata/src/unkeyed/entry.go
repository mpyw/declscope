package unkeyed

type Entry struct {
	// ID is exported, so it resolves to package and carries no boundary.
	ID int
	// count is unexported, so it is bounded by the namespace declaring Entry.
	// An unkeyed literal writes it without naming it, and that is still a use.
	count int // want `field Entry.count is private to namespace "entry", but is used from namespace "build"`
	// note is written by the same literal. An unkeyed literal writes every
	// field, so there is no writing one without the others.
	note string // want `field Entry.note is private to namespace "entry", but is used from namespace "build"`
}

func entryMake() Entry { return Entry{ID: 1, count: 2, note: "x"} }

var _ = entryMake

// A generic struct is written the same way. The literal's type is an
// instantiation, whose fields are mapped back to the ones declared here.
type Pair[T any] struct {
	ID  int
	val T // want `field Pair.val is private to namespace "entry", but is used from namespace "build"`
}
