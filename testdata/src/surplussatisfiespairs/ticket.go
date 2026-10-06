package surplussatisfiespairs

//declscope:shared
type ticket struct{}

//declscope:shared
func (ticket) fetch() int { return 1 }
