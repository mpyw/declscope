package qualifyalways

// The package has a single namespace, so the default would not require a
// prefix here. rules.qualify: always asks for it anyway.
func helper() int { return 1 } // want `func helper does not carry namespace "only" anywhere in its name; move it to the file it belongs to, or give it a name that carries "only" \(onlyHelper is one candidate, not necessarily the best\)`

func onlyOK() int { return helper() }

var _ = onlyOK
