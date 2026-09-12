package baselined

// helper is recorded in the baseline, so it stays quiet.
func helper() int { return 1 }

// fresh is not, so it is reported.
func fresh() int { return 2 } // want `func fresh is file-private to namespace "user", but is used from namespace "order"`

type User struct {
	// name is recorded in the baseline.
	name string
}

// note is not.
var note = "x" // want `var note is file-private to namespace "user", but is used from namespace "order"`
