package lineforeign

// A foreign method: its receiver's type is declared in user.go.
func (u *userRecord) leak() string { // want `method leak does not carry namespace "order" anywhere in its name; give it a name that carries "order", such as orderLeak, or move it to the file it belongs to`
	return u.name
}

func orderRun() string { return (&userRecord{}).leak() }

var _ = orderRun
