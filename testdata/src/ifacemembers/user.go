package ifacemembers

// An interface declares its method names, and only this package can spell an
// unexported one. That is the sealed-interface idiom, and the boundary gives
// it file granularity.
type Sealed interface {
	sealed() bool // want `method Sealed.sealed is private to namespace "user", but is used from namespace "order"`
	// Exported: published to every importer, so no boundary by default.
	Open() int
}

// The type's directive reaches its methods, the way it reaches a struct's
// fields: both are written inside the declaration.
//
//declscope:package
type Widened interface {
	shared() int
}

// A method may state its own scope, which outranks the type's.
type Mixed interface {
	//declscope:package
	ok() int
	nope() int // want `method Mixed.nope is private to namespace "user", but is used from namespace "order"`
}

// The type's ignore covers its methods too.
//
//declscope:ignore boundary
type Quiet interface {
	hushed() int
}

// Generic interfaces are no different.
type Store[T any] interface {
	fetch(id int) T // want `method Store.fetch is private to namespace "user", but is used from namespace "order"`
}

// Nothing here declares a name of its own: an embedded interface takes the
// embedded type's names, and a type-constraint element has none at all.
type Combined interface {
	Sealed
	Store[int]
}

type Number interface{ ~int | ~float64 }
