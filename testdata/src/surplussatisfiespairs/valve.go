package surplussatisfiespairs

// shut is a function, and shutter in use.go asks for a method of that name.
// A contract reaches methods alone, so no contract reaches the function, and
// its directive is reported.
//
//declscope:package // want `//declscope:package on shut: no use from another namespace is visible to declscope`
func shut() {}

var _ = shut
