//declscope:namespace user

package typeignore

// Declared in another file, and still covered by the directive on User.
func (u *User) normalize() { u.name = "x" }
