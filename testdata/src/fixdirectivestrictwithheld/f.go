package fixdirectivestrictwithheld

// The block binds nothing, but fNarrowed needs it, so loose alone reports it.
// Deleting FExported's directive would hand FExported to the block, and the
// block's report would name it.
//
//declscope:package // want `unused //declscope:package: every declaration it reaches states its own scope`
var (
	//declscope:package // want `unused //declscope:package on FExported: it already has package scope`
	FExported = 1
	//declscope:private
	fNarrowed = 2
)

var _ = fNarrowed
