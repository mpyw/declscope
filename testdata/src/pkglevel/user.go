package pkglevel

// helper is not prefixed with the file's namespace, so it is file-private.
func helper() int { return 1 } // want `func helper is file-private to namespace "user", but is used from namespace "order"`

// userShared carries the namespace prefix, so it is package-internal.
func userShared() int { return 2 }

// user matches the namespace exactly, which also counts as prefixed.
func user() int { return 3 }

// users is not prefixed: the character after the prefix must start a new word.
func users() int { return 4 } // want `func users is file-private to namespace "user", but is used from namespace "order"`

var count int // want `var count is file-private to namespace "user", but is used from namespace "order"`

var userTotal int

const limit = 10 // want `const limit is file-private to namespace "user", but is used from namespace "order"`

// Exported is public and may be used from anywhere.
func Exported() int { return helper() }

type payload struct{} // want `type payload is file-private to namespace "user", but is used from namespace "order"`
