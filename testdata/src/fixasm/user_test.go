package fixasm_test

// user.s is built into package fixasm only, so it names nothing in this
// external test package, which keeps its rename and its surplus report.

//declscope:shared // want `//declscope:shared on userThing: no use from another namespace is visible to declscope`
var userThing = 1

func plain() int64 { return 2 } // want `func plain does not carry namespace "user" anywhere in its name; rename it to userPlain, or to another name that carries "user"`
