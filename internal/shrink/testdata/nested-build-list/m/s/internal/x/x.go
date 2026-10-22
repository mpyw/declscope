package x

// Foo is what the copy of t that the nested module n builds with uses.
// Nothing in this tree does, and unexporting it breaks go build ./... of n.
func Foo() {}

// Bar is used by nothing, but its range is not judged either.
func Bar() {}
