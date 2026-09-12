package prefixalways

// The package has a single namespace, so the default would not require a
// label here. rules.prefix: true asks for it anyway.
func helper() int { return 1 } // want `func helper does not carry the prefix of namespace "only"; rename it to onlyHelper`

func onlyOK() int { return helper() }

var _ = onlyOK
