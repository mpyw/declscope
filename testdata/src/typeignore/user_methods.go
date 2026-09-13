//declscope:namespace user

package typeignore

// The directive on User does not reach here: a method needs its own. Without
// one this is reported, which is what pins the type's ignore to what the type
// contains.
func (u *User) normalize() { u.name = "x" } // want `method User.normalize is private to namespace "user", but is used from namespace "order"`
