package fixasm

// user.s defines add, and the linker finds it by this name.
func add(a, b int64) int64 // want `func add does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userAdd is one candidate, not necessarily the best\)`

// Nothing names plain from assembly, but the package holds it, so plain is
// not renamed either.
func plain() int64 { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userPlain is one candidate, not necessarily the best\)`

func Exported() int64 { return add(1, 2) + plain() }
