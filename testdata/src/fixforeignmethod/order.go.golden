package fixforeignmethod

// orderT holds a field named like one rename below would name a method.
//
//declscope:shared
type orderT struct {
	//declscope:private
	userSize int
}

// orderOuter promotes every method of orderT, and declares a field of its own.
//
//declscope:shared
type orderOuter struct {
	orderT
	//declscope:private
	userDepth int
}

// An unnamed struct promotes the methods too.
var orderAnon struct {
	orderT
	userWidth int
}

type orderHelper interface{ help() int }

type orderChecker interface{ userCheck() bool }

// orderT satisfies orderHelper statically.
var _ orderHelper = orderT{}

// orderAsserted asks only through type assertions, one to a named interface
// and one to an unnamed one. Renaming a method would still compile, but flip
// what an assertion answers.
func orderAsserted() (bool, bool) {
	_, checked := any(orderT{}).(orderChecker)
	_, asserted := any(orderT{}).(interface{ assert() int })
	return checked, asserted
}

var _, _ = orderOuter{}, orderAnon
