package fixtypeownsharedunusedoff

// entry crosses, and so does key. note states //declscope:shared of its own.
// With the unused rule off, widening entry reports nothing about note's
// directive, so entry's fix is offered and key's is subsumed by it.
type entry struct { // want `type entry is private to namespace "user", but is used from namespace "order"`
	key string // want `field entry.key is private to namespace "user", but is used from namespace "order"`
	//declscope:shared // order.go reads it
	note string
}
