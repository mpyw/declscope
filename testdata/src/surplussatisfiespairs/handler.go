package surplussatisfiespairs

// A named function type has methods although its underlying type cannot.
//
//declscope:package
type handlerFunc func() string

//declscope:package
func (f handlerFunc) serve() string { return f() }
