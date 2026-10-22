package fixasmskipped

// The go command skips _old.s, as it skips every name that starts with _ or
// ., so plain is still renamed.
func plain() int64 { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; rename it to userPlain, or to another name that carries "user"`

func Exported() int64 { return plain() }
