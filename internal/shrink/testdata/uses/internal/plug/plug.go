// Package plug is held by a variable of package main, which a plugin host
// looks up, so Plug's method and field are used there.
package plug

type Plug struct {
	F int
}

func (Plug) M() {}
