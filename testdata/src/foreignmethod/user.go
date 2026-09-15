package foreignmethod

type userRecord struct { // want `type userRecord is private to namespace "user", but is used from namespace "order"`
	name string // want `field userRecord.name is private to namespace "user", but is used from namespace "order"`
}

// A local method is read through a receiver that names the unit holding it, so
// the naming rule asks nothing of it.
func (u *userRecord) bump() string { // want `method userRecord.bump is private to namespace "user", but is used from namespace "order"`
	return u.name
}

// An interface method name is a member, not a method with a receiver.
type userSealed interface { // want `type userSealed is private to namespace "user", but is used from namespace "order"`
	seal()
}

func userMake() *userRecord { return &userRecord{} } // want `func userMake is private to namespace "user", but is used from namespace "order"`
