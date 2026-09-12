package directives

//declscope:package
func userExplicitlyShared() int { return 1 }

//declscope:file
func userNotReallyShared() int { return 2 } // want `func userNotReallyShared is declared file-private by //declscope:file, but is used from namespace "order"`

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
	userPrivateB = 2 // want `var userPrivateB is file-private to namespace "user", but is used from namespace "order"`
)
