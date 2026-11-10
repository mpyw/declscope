package nsnormalize

// A stem that opens with a plural initialism is lowered whole, as Go lowers
// the identifier URLs, so the suggested name is urlsHelper rather than
// urLsHelper.
func helper() int { return 1 } // want `func helper does not carry namespace "urls" anywhere in its name; move it to the file it belongs to, or give it a name that carries "urls" \(urlsHelper is one candidate, not necessarily the best\)`

func urlsOK() int { return helper() }
