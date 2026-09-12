package fixes

func userHelper() int { return 1 } // want `func userHelper is private to namespace "user", but is used from namespace "order"`

var (
	userSeed = 2 // want `var userSeed is private to namespace "user", but is used from namespace "order"`
)
