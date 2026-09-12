package memberowner

// Written here, but owned by Auth, which lives in 2fa.go. The boundary is the
// type's namespace, so the message must name "2fa": naming this file's would
// make it read "private to namespace "order", but is used from namespace
// "order"".
func (a *Auth) helper() int { return a.secret } // want `method Auth.helper is private to namespace "2fa", but is used from namespace "order"`

func orderRun(a *Auth) int { return a.helper() }

var _ = orderRun
