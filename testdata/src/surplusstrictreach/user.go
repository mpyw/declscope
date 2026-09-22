package surplusstrictreach

// Every field below is reached from order.go by a path that spells nothing
// or spells it through something else. Each one keeps the type's scope.

// A composite literal without keys writes both fields by position.
//
//declscope:package
type userRec struct {
	a int
	b int
}

// A selection through an embedding names the field of the embedded type.
//
//declscope:package
type userInner struct {
	n int
}

// A selection on an instantiation names the instantiated field, which is
// mapped back to the declared one.
//
//declscope:package
type userList[T any] struct {
	items []T
}

// A struct conversion pairs every field by name and spells none of them.
//
//declscope:package
type userModel struct {
	rev int
}

// A method written in order.go reads the field from there.
//
//declscope:package
type userBox struct {
	v int
	// stray is read by nothing outside, which shows the guards above are
	// precise rather than off.
	stray int // want `field userBox.stray takes package scope from //declscope:package on userBox, but no use from another namespace is visible to declscope`
}

func userTouch(b userBox) int { return b.stray }

var _ = userTouch
