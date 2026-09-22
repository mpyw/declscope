package fixes

func userHelper() int { return 1 } // want `func userHelper is private to namespace "user", but is used from namespace "order"`

var (
	userSeed = 2 // want `var userSeed is private to namespace "user", but is used from namespace "order"`
)

// A type block is anchored a spec at a time, the way a var block is: the
// directive lands on the type that crosses, and the one beside it keeps the
// scope it had.
type (
	userBox  struct{ n int } // want `type userBox is private to namespace "user", but is used from namespace "order"`
	userCase struct{ n int }
)

var _ userCase
