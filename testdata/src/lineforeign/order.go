package lineforeign

// A foreign method: its receiver's type is declared in user.go.
func (u *userRecord) leak() string { // want `method leak does not carry namespace "order" anywhere in its name; rename it to orderLeak, or to another name that carries "order"`
	return u.name
}

func orderRun() string { return (&userRecord{}).leak() }

var _ = orderRun
