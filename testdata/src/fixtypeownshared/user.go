package fixtypeownshared

// entry crosses, and so does key. note states //declscope:shared of its own,
// which binds only while entry states no scope: widened, entry would give note
// that scope anyway, and the unused rule would report the directive the author
// wrote. So entry's fix is withheld, and key, which nothing widens now, takes
// its own.
type entry struct { // want `type entry is private to namespace "user", but is used from namespace "order"`
	key string // want `field entry.key is private to namespace "user", but is used from namespace "order"`
	//declscope:shared // order.go reads it
	note string
}

// Label states //declscope:shared too, but it is exported, so the directive
// binds nothing before the fix either. The fix on pair is offered.
type pair struct { // want `type pair is private to namespace "user", but is used from namespace "order"`
	//declscope:shared // want `unused //declscope:shared on pair.Label: it already has shared scope`
	Label string
}
