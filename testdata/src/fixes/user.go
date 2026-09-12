package fixes

func helper() int { return 1 } // want `func helper is file-private to namespace "user", but is used from namespace "order"`

var (
	seed = 2 // want `var seed is file-private to namespace "user", but is used from namespace "order"`
)
