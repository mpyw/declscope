package fixsurplusnarrow

// entry crosses, and the boundary fix widens it. note is read here alone, so
// the same edit narrows it back, or surplus would report what -fix wrote.
// memo shares its line, so one directive narrows both, and extra takes its
// own.

type entry struct { // want `type entry is private to namespace "user", but is used from namespace "order"`
	key        string // want `field entry.key is private to namespace "user", but is used from namespace "order"`
	note, memo string
	extra      int
}

func userNote(e entry) string { return e.note + e.memo + string(rune(e.extra)) }

var _ = userNote
