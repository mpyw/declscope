package memberowner

// Written here, but owned by Auth, which lives in 2fa.go. The boundary is the
// type's file, so the message must name 2fa.go: naming this file would make it
// read "private to file order.go, but is used from namespace "order"".
func (a *Auth) helper() int { return a.secret } // want `method Auth.helper is private to file 2fa.go, but is used from namespace "order"`

func orderRun(a *Auth) int { return a.helper() }

var _ = orderRun
