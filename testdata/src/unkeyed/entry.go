package unkeyed

type Entry struct {
	// ID is exported, so it resolves to package and carries no boundary.
	ID int
	// count is unexported, so it is bounded by the namespace declaring Entry.
	// An unkeyed literal writes it without naming it, and that is still a use.
	count int // want `field Entry.count is private to namespace "entry", but is used from namespace "build"`
	// note is written only from a keyed literal inside this namespace.
	note string
}

func entryMake() Entry { return Entry{ID: 1, count: 2, note: "x"} }

var _ = entryMake
