package surplussatisfies

//declscope:shared
type closer struct{}

//declscope:shared
func (closer) close() string { return "c" }
