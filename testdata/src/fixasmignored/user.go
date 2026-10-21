package fixasmignored

// user_s390x.s calls double, in a build this pass does not see.
func double(a int64) int64 { return 2 * a } // want `func double does not carry namespace "user" anywhere in its name; rename it to userDouble, or to another name that carries "user"`

func Exported() int64 { return double(1) }
