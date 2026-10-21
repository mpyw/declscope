package nsnormalize

// A stem that opens with a plural initialism is lowered whole, as Go lowers
// the identifier URLs, so the suggested name is urlsHelper rather than
// urLsHelper.
func helper() int { return 1 } // want `func helper does not carry namespace "urls" anywhere in its name; rename it to urlsHelper, or to another name that carries "urls"`

func urlsOK() int { return helper() }
