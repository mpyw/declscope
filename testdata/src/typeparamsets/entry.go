package typeparamsets

// Each type below is written by build.go in one shape only, through a type
// parameter, and the literal or conversion spells none of its fields.

// Named is the constraint itself: P interface{ Named }. Its type set holds one
// struct, so P{1} and []P{{1}} write count.
type Named struct {
	count int // want `field Named.count is private to namespace "entry", but is used from namespace "build"`
}

// Narrow and Wide are both in a union, which a second element intersects
// with ~*Narrow alone. Only Narrow is left, so []P{{1}} writes Narrow.count.
type Narrow struct {
	count int // want `field Narrow.count is private to namespace "entry", but is used from namespace "build"`
}

type Wide struct {
	count int
}

// Via is reached through an interface term of a union, PtrTo[Via] | *Via.
type Via struct {
	count int // want `field Via.count is private to namespace "entry", but is used from namespace "build"`
}

type PtrTo[T any] interface{ *T }

// Left and Right have one underlying type, so P interface{ Left | Right }
// has it as its core type, and P{1} writes the fields of both.
type Left struct {
	count int // want `field Left.count is private to namespace "entry", but is used from namespace "build"`
}

type Right struct {
	count int // want `field Right.count is private to namespace "entry", but is used from namespace "build"`
}

// Converted is the constraint of P, and P(u) pairs its field with u's by name.
// The pairing is what the directive holds up, so it is not surplus.
type Converted struct {
	//declscope:shared
	count int
}

var _ = Wide{count: 1}

var _ = Converted{count: 1}
