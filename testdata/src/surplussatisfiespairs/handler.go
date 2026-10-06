package surplussatisfiespairs

// A named function type has methods although its underlying type cannot.
//
//declscope:shared
type handlerFunc func() string

//declscope:shared
func (f handlerFunc) serve() string { return f() }
