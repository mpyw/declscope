package qualifydefault

// No config reaches this package, so the naming rule is at its default: off.
// helper carries no namespace and draws no qualify diagnostic even though the
// package has a second namespace. The boundary rule is untouched by the
// default and still fires below.
func helper() int { return 1 } // want `func helper is private to namespace "user", but is used from namespace "order"`

// bare is used only from its own namespace: no boundary, and no qualify.
func bare() int { return helper() }

var _ = bare
