// Package a is imported by the nested module in tools, whose package q holds
// a value of T in a compiled file. A file of q that its build excludes imports
// nothing, but may still call T's method through that value.
package a

type T struct{}

func New() T { return T{} }

func (T) Method() int { return 0 } // want: method Method is exported.*no fix: a build-excluded file of another package may use it

func Unused() {} // want: func Unused is exported, but nothing outside example.com/nx/internal/a uses it$
