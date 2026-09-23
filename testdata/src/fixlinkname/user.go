package fixlinkname

import _ "unsafe"

// The directive names helper as text, which a rename cannot follow.
//
//go:linkname helper
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; rename it to userHelper, or to another name that carries "user"`

// A cgo //export names tick as text the same way.
//
//export tick
func tick() int { return 3 } // want `func tick does not carry namespace "user" anywhere in its name; rename it to userTick, or to another name that carries "user"`

func plain() int { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; rename it to userPlain, or to another name that carries "user"`

func Exported() int { return helper() + tick() + plain() }
