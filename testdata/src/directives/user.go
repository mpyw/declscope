package directives

//declscope:package
func explicitlyShared() int { return 1 }

//declscope:file
func userNotReallyShared() int { return 2 } // want `func userNotReallyShared is declared file-private by //declscope:file, but is used from namespace "order"`

//declscope:ignore
func ignoredLeak() int { return 3 }

//declscope:ignore // want `unused //declscope:ignore on neverLeaks`
func neverLeaks() int { return 4 }

//declscope:bogus // want `unknown directive declscope:bogus`
func userTypo() int { return 5 }

func trailing() int { return 6 } //declscope:package // shared with the reporting code

var (
	//declscope:package
	sharedA  = 1
	privateB = 2 // want `var privateB is file-private to namespace "user", but is used from namespace "order"`
)
