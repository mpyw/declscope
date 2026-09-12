package typeignore

// A directive on the type covers everything the type owns: its fields, and its
// methods wherever they are declared.
//
//declscope:ignore boundary
type User struct {
	name string
	id   int
}

// Open carries a directive of its own as well as inheriting the type's. Both
// count as used, so neither is reported.
//
//declscope:ignore boundary
type Open struct {
	secret string
}

//declscope:ignore unqualify // want `unused //declscope:ignore unqualify on Closed`
type Closed struct {
	// The type's directive names a rule that never fires, so the field is
	// still reported.
	hidden string // want `field Closed.hidden is private to namespace "user", but is used from namespace "order"`
}
