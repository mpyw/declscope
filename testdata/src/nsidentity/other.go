package nsidentity

// The second namespace, which is what puts qualify in force for the package.
// It is a prefix, so its own declarations are held to it.
func otherRun() int { return 1 }

func helper() int { return 2 } // want `func helper does not carry namespace "other" anywhere in its name; rename it to otherHelper, or to another name that carries "other"`

// A cross-namespace use is still a violation: 2fa is an identity even though
// it is not a prefix.
var _ = totp

var _ = otherRun() + helper()
