package surplussatisfiespairs

// Each contract below is the only way its methods are reached. The rule may
// skip an interface or a type before testing satisfaction, and these are the
// shapes such a skip could get wrong.

// A method promoted through an embedded pointer. Only wrap has both methods:
// spin comes from *engine and stop from brake, so neither type satisfies
// the interface alone.
type spinStopper interface {
	spin()
	stop()
}

type wrap struct {
	*engine
	brake
}

var _ spinStopper = wrap{}

// A method of a generic type, satisfied by an instantiation alone. poke has
// a pointer receiver, so only the instantiation's address satisfies.
type peekPoker interface {
	peek() string
	poke(string)
}

var _ peekPoker = &cell[string]{}

// An interface embedding another. tock is asked for by tickTocker alone, and
// tick by both.
type ticker interface{ tick() }

type tickTocker interface {
	ticker
	tock()
}

var _ tickTocker = clock{}

// A requirement that only an embedded interface asks for. fetcherOf declares
// no method of its own, and the instantiation that asks fetch() int is never
// written: it exists only inside fetcherOf[int]. No other interface of the
// package asks for fetch with that signature.
type getterOf[T any] interface{ fetch() T }

type fetcherOf[T any] interface{ getterOf[T] }

var _ fetcherOf[int] = ticket{}

// A named type whose underlying type has no methods. Only the named type
// satisfies, and its address is never written.
type server interface{ serve() string }

var _ server = handlerFunc(nil)
