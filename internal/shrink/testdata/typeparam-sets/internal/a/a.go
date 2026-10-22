// Package a declares structs that package b writes through a type parameter
// only: each literal or conversion there spells none of their fields. Each
// struct is written in one shape.
package a

// Named is the constraint itself: P interface{ a.Named }.
type Named struct{ X, Y int }

// Converted is the constraint of P, and P(u) pairs its fields with u's.
type Converted struct{ X, Y int }

// Narrow and Wide are both in a union, which a second element intersects
// with ~*Narrow alone, so only Narrow's fields are written.
type Narrow struct{ X, Y int }

type Wide struct {
	X int // want: field X is exported, but nothing.*uses it$
	Y int // want: field Y is exported, but nothing.*uses it$
}

// Via is reached through an interface term of a union.
type Via struct{ X, Y int }

// Left and Right share one underlying type, so a literal of
// P interface{ a.Left | a.Right } writes the fields of both.
type Left struct{ X, Y int }

type Right struct{ X, Y int }
