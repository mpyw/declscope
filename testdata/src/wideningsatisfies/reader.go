package wideningsatisfies

// The type is spelled from use.go, which keeps its own directive alive. The
// methods are reached only through interface contracts.
//
//declscope:package
type reader struct{}

//declscope:package
func (reader) read() string { return "r" }

// stray participates in no contract, so its directive is reported.
//
//declscope:package // want `//declscope:package on reader.stray: no use from another namespace is visible to declscope`
func (reader) stray() string { return "s" }
