package nsidentity

// The second namespace, which is what puts promote in force for the package.
// It is a label, so its own declarations are held to it.
func otherRun() int { return 1 }

func helper() int { return 2 } // want `func helper does not carry the prefix of namespace "other"; rename it to otherHelper`

// A cross-namespace use is still a violation: 2fa is an identity even though
// it is not a label.
var _ = totp

var _ = otherRun() + helper()
