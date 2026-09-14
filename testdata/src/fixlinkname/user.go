package fixlinkname

import _ "unsafe"

// The directive names helper as text, which a rename cannot follow.
//
//go:linkname helper
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; rename it to userHelper`

func plain() int { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; rename it to userPlain`

func Exported() int { return helper() + plain() }
