package unkeyed

type Entry struct {
	// ID is exported, so it resolves to shared and carries no boundary.
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

// A literal inside another may leave out its type, a pointer one too. In
// []*Slot{{1}} the inner literal has the type *Slot, and writes the fields of
// the struct it points to. Each type below is written in one form only.
type Slot struct {
	count int // want `field Slot.count is private to namespace "entry", but is used from namespace "build"`
}

type Cell struct {
	count int // want `field Cell.count is private to namespace "entry", but is used from namespace "build"`
}

type Key struct {
	count int // want `field Key.count is private to namespace "entry", but is used from namespace "build"`
}

type Value struct {
	count int // want `field Value.count is private to namespace "entry", but is used from namespace "build"`
}

type Deep struct {
	count int // want `field Deep.count is private to namespace "entry", but is used from namespace "build"`
}

// A keyed literal names its fields, so its type does not matter.
type Named struct {
	count int // want `field Named.count is private to namespace "entry", but is used from namespace "build"`
}
