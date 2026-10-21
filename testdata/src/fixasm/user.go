package fixasm

// user.s defines add, and the linker finds it by this name.
func add(a, b int64) int64 // want `func add does not carry namespace "user" anywhere in its name; rename it to userAdd, or to another name that carries "user"`

// Nothing names plain from assembly, but the package holds it, so plain is
// not renamed either.
func plain() int64 { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; rename it to userPlain, or to another name that carries "user"`

func Exported() int64 { return add(1, 2) + plain() }
