package fixes

func userHelper() int { return 1 } // want `func userHelper is file-private to namespace "user", but is used from namespace "order"`

var (
	userSeed = 2 // want `var userSeed is file-private to namespace "user", but is used from namespace "order"`
)
