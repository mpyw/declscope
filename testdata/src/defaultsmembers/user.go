package defaultsmembers

func Helper() int { return 1 } // want `func Helper is private to namespace "user", but is used from namespace "order"`

type User struct { // want `type User is private to namespace "user", but is used from namespace "order"`
	Name string // want `field User.Name is private to namespace "user", but is used from namespace "order"`
}

func (u *User) Bump() {} // want `method User.Bump is private to namespace "user", but is used from namespace "order"`
