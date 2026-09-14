package typeignore

// A directive on the type covers what the type contains: its fields, which are
// written inside this declaration. It does not reach its methods — a method is
// an ordinary top-level declaration, and the suppression chain walks the same
// levels as scope resolution, so that learning one teaches the other.
//
//declscope:ignore boundary
type User struct {
	name string
	id   int
}

// Open carries a directive of its own as well as the one on its type. Both
// count as used, so neither is reported.
//
//declscope:ignore boundary
type Open struct {
	secret string
}

//declscope:ignore qualify // want `unused //declscope:ignore qualify on Closed`
type Closed struct {
	// The type's directive names a rule that never fires, so the field is
	// still reported.
	hidden string // want `field Closed.hidden is private to namespace "user", but is used from namespace "order"`
}
