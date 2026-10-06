package surplussatisfies

// The type is spelled from use.go, which keeps its own directive alive. The
// methods are reached only through interface contracts.
//
//declscope:shared
type reader struct{}

//declscope:shared
func (reader) read() string { return "r" }

// stray participates in no contract, so its directive is reported.
//
//declscope:shared // want `//declscope:shared on reader.stray: no use from another namespace is visible to declscope`
func (reader) stray() string { return "s" }
