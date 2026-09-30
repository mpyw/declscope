// Package x has a build-excluded file of its own. That file is not
// type-checked, so a field or a method of x may be used there without its
// name showing: satisfying an interface, or matched in a conversion. So no
// field or method of x is fixed, and a name the file writes is not either.
package x

// WinRef is named by the build-excluded file, which the rename would leave
// behind.
var WinRef = 1 // want: var WinRef is exported.*no fix: a build-excluded file of its package names it

// Unnamed is written nowhere, and a package-level name has no use that does
// not spell it, so it is fixed.
func Unnamed() {} // want: func Unnamed is exported, but nothing.*uses it$

// Own is written nowhere in the build-excluded file, which only asks for an
// io.Closer that newOwn returns, but that is enough to need Close.
type Own struct { // want: type Own is exported, but nothing.*uses it$
	F int // want: field F is exported.*no fix: a build-excluded file of its package may use it
}

func (Own) Close() error { return nil } // want: method Close is exported.*no fix: a build-excluded file of its package may use it

func newOwn() Own { return Own{} }

var _ = Own{}.F
