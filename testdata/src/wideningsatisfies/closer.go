package wideningsatisfies

//declscope:package
type closer struct{}

//declscope:package
func (closer) close() string { return "c" }
