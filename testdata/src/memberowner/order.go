package memberowner

// A method is an ordinary top-level declaration: written here, it belongs to
// this file's namespace, so this file may use it and the call below is not a
// crossing. What it may not do is reach into Auth's fields, which belong to the
// file that declares them — and that is the boundary doing the work. Binding
// the method itself to 2fa.go instead would have made it unusable from the file
// that wrote it, with no scope able to say otherwise.
func (a *Auth) helper() int { return a.secret }

func orderRun(a *Auth) int { return a.helper() }

var _ = orderRun
