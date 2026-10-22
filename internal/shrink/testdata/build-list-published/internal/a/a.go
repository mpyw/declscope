package a

// New is what the nested module ext uses.
func New() {}

// Old is what the published ext v1.0.0, which the build reads, uses.
// Nothing in this tree does, and unexporting it breaks go build ./... of
// this module.
func Old() {}
