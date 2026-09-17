package fixmembers

// Both the type and its field cross, and a directive on the type reaches the
// field. Only the wider one is written: a directive on the field as well would
// bind nothing, and the directive rule would report what -fix had just
// written.
type entry struct { // want `type entry is private to namespace "user", but is used from namespace "order"`
	key string // want `field entry.key is private to namespace "user", but is used from namespace "order"`
}

// Here the type itself does not cross — another namespace reaches a value of
// it without ever naming it — so nothing widens the field but its own
// directive, and the fix is offered.
type kept struct {
	flag bool // want `field kept.flag is private to namespace "user", but is used from namespace "order"`
}

func UserMake() kept { return kept{flag: true} }
