package directives

//declscope:package
func userExplicitlyShared() int { return 1 }

//declscope:private
func userNotReallyShared() int { return 2 } // want `func userNotReallyShared is declared private by //declscope:private, but is used from namespace "order"`

//declscope:ignore
func userIgnoredLeak() int { return 3 }

//declscope:ignore // want `unused //declscope:ignore on userNeverLeaks`
func userNeverLeaks() int { return 4 }

//declscope:bogus // want `unknown directive declscope:bogus`
func userTypo() int { return 5 }

func userTrailing() int { return 6 } //declscope:package // shared with the reporting code

var (
	//declscope:package
	userSharedA  = 1
	userPrivateB = 2 // want `var userPrivateB is private to namespace "user", but is used from namespace "order"`
)

// A scope directive on the block reaches every spec, and one on a spec
// replaces it: a declaration has exactly one scope.
//
//declscope:package
var (
	userSharedC = 3
	//declscope:private
	userPrivateD = 4 // want `var userPrivateD is declared private by //declscope:private, but is used from namespace "order"`
)
