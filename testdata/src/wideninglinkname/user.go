package wideninglinkname

import _ "unsafe"

// userClock is reached through the linkname as text, never through a spelled
// reference.
//
//go:linkname userClock
//declscope:package
func userClock() int64 { return 0 }

// userTick is named by an //export directive the same way.
//
//export userTick
//declscope:package
func userTick() int32 { return 1 }

// userQuiet is named by neither, so its directive is reported.
//
//declscope:package // want `//declscope:package on userQuiet: no use from another namespace is visible to declscope`
func userQuiet() int { return 2 }
