package unusedstrict

// The exported spec's package widens it back from its block's private, so
// strict keeps it. The block is kept as well: without it, the spec's package
// would restate what exportedness gives.
//
//declscope:private
var (
	//declscope:package
	GWidened = 2
	gOther   = 3
)

var _ = gOther
