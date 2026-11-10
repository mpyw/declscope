package fixasmignored

// user_s390x.s calls double, in a build this pass does not see.
func double(a int64) int64 { return 2 * a } // want `func double does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userDouble is one candidate, not necessarily the best\)`

func Exported() int64 { return double(1) }
