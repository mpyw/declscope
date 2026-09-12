package nsidentity

// The stem 2fa is a namespace like any other, so 2fa_test.go shares it and
// may use totp. It cannot be a label, since no identifier starts with a
// digit, so qualify asks nothing of totp even though the package has a second
// namespace; the privacy rules still bound it, so the use from other.go is
// reported here.
func totp() int { return 1 } // want `func totp is private to namespace "2fa", but is used from namespace "other"`

func Verify() int { return totp() }
