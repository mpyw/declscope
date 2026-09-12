package members

type User struct {
	// ID is exported and therefore public.
	ID int
	// name is unexported, so it is bounded by the namespace declaring User.
	name string // want `field User.name is private to namespace "user", but is used from namespace "order"`
	// note is only touched inside this namespace.
	note string
}

func (u *User) normalize() { // want `method User.normalize is private to namespace "user", but is used from namespace "order"`
	u.note = u.name
}

// Name is exported and therefore public.
func (u *User) Name() string { return u.name }

//declscope:package
func userMake(name string) *User {
	u := &User{name: name}
	u.normalize()
	return u
}
