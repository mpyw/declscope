package baselined

// userHelper is recorded in the baseline, so it stays quiet.
func userHelper() int { return 1 }

// userFresh is not, so it is reported.
func userFresh() int { return 2 } // want `func userFresh is file-private to namespace "user", but is used from namespace "order"`

type User struct {
	// name is recorded in the baseline.
	name string
	// note is not.
	note string // want `field User.note is private to namespace "user", but is used from namespace "order"`
}
