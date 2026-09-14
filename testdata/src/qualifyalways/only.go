package qualifyalways

// The package has a single namespace, so the default would not require a
// prefix here. rules.qualify: always asks for it anyway.
func helper() int { return 1 } // want `func helper does not carry namespace "only" anywhere in its name; rename it to onlyHelper`

func onlyOK() int { return helper() }

var _ = onlyOK
