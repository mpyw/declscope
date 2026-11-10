package fixasmskipped

// The go command skips _old.s, as it skips every name that starts with _ or
// ., so plain is still renamed.
func plain() int64 { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userPlain, or move it to the file it belongs to`

func Exported() int64 { return plain() }
