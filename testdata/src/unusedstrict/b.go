package unusedstrict

// A block's directive that restates the default is reported with every spec
// that takes it.
//
//declscope:private // want `unused //declscope:private on seed, limit: each already has private scope`
var (
	seed  = 1
	limit = 2
)

// A spec restating its block is reported. The block keeps its report too: it
// names what every spec already has.
//
//declscope:private // want `unused //declscope:private: every declaration it reaches already has private scope`
var (
	//declscope:private // want `unused //declscope:private on other: it already has private scope`
	other = 3
)

// A spec narrowing its block's package is not redundant: without it the spec
// would take the block's scope, not the default.
//
//declscope:package
var (
	//declscope:private
	narrowed = 4
	wide     = 5
)

var _ = seed + limit + other + narrowed + wide
