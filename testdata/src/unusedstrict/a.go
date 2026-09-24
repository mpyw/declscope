package directivestrict

// Under strict, a private field of a type that states no scope is reported:
// defaults.unexported gives it private without the directive.
type implicit struct {
	//declscope:private // want `unused //declscope:private on implicit.x: it already has private scope`
	x int
}

// The type's directive restates the default too, and the field's restates the
// type's. Both are reported, as loose reports the field's.
//
//declscope:private // want `unused //declscope:private on explicit: it already has private scope`
type explicit struct {
	//declscope:private // want `unused //declscope:private on explicit.x: it already has private scope`
	x int
}

// A directive that changes the scope is kept: package widens an unexported
// declaration, and private narrows an exported one.
//
//declscope:package
func widened() int { return 1 }

//declscope:private
func Narrowed() int { return 2 }

// A type's directive that one field overrides is judged by that field too:
// without the type's, the field's own would restate the default instead.
//
//declscope:package
type mixed struct {
	shared int
	//declscope:private
	own int
}

var _ = implicit{}.x + explicit{}.x + widened() + mixed{}.shared + mixed{}.own
