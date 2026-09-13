package defaultsmembers

// The configured default reaches fields and methods, not only package-level
// declarations, so a key that says "package" governs every declaration here
// rather than half of them. A directive on one of them still overrides it.
type User struct {
	name string

	//declscope:private
	secret string // want `field User.secret is declared private by //declscope:private, but is used from namespace "order"`
}

func (u *User) bump() {}

//declscope:private
func (u *User) seal() {} // want `method User.seal is declared private by //declscope:private, but is used from namespace "order"`
