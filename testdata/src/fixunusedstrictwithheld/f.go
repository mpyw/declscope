package fixunusedstrictwithheld

// The block binds nothing, but fNarrowed needs it, so loose alone reports it.
// Deleting FExported's directive would hand FExported to the block, and the
// block's report would name it.
//
//declscope:shared // want `unused //declscope:shared: every declaration it reaches states its own scope`
var (
	//declscope:shared // want `unused //declscope:shared on FExported: it already has shared scope`
	FExported = 1
	//declscope:private
	fNarrowed = 2
)

var _ = fNarrowed
