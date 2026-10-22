package z

// Baz is what the copy of u that the nested module g builds with under the
// never tag uses. Nothing in this tree does, and unexporting it breaks
// go build -tags=never ./... of g.
func Baz() {}
