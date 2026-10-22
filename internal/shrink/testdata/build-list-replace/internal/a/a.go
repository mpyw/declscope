package a

// New is what the nested module ext uses.
func New() {}

// Used is what the copy of ext the build reads uses. Nothing in this tree
// does, and unexporting it breaks go build ./... of this module.
func Used() {}
