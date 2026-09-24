package unusedloose

// The exported spec's package widens it back from its block's private.
// Deleting it would narrow the spec, so it binds. The block binds cOther.
//
//declscope:private
var (
	//declscope:package
	CWidened = 2
	cOther   = 3
)

var _ = cOther
