package fixexcludedlinkname

// helper is named by a //go:linkname in excluded.go, and tick by an //export
// in other_windows.go. The build configuration leaves both files out, so the
// fix cannot rewrite them. Renamed, the build that keeps excluded.go fails
// with "//go:linkname must refer to declared function or variable". Both
// renames are withheld.
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userHelper is one candidate, not necessarily the best\)`

func tick() int { return 3 } // want `func tick does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userTick is one candidate, not necessarily the best\)`

// Named nowhere else, so the rename is offered.
func plain() int { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userPlain is one candidate, not necessarily the best\)`

func Exported() int { return helper() + tick() + plain() }
