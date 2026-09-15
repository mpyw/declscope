package surpluscarrier

// solo is unexported, so nothing carries quiet out, and the directive is
// reported.
type solo struct{}

//declscope:package // want `//declscope:package on solo.quiet: no use from another namespace is visible to declscope`
func (solo) quiet() {}
