// Package s is imported by package t. t's build-excluded file imports
// nothing, but reads a value of s through a variable its compiled sibling
// declares. That is enough to need any field or method of s, so none is
// fixed.
package s

type S struct {
	Field int // want: field Field is exported.*no fix: a build-excluded file of another package may use it
}

func (S) Method() string { return "" } // want: method Method is exported.*no fix: a build-excluded file of another package may use it

func New() S { return S{} }
