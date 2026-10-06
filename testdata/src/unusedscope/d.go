package unusedscope

// A block's directive that one spec overrides and another spec takes without
// needing it is not overridden by every spec. The report names the spec that
// takes it: Exported is shared under every configuration.
//
//declscope:shared // want `unused //declscope:shared on Exported: nothing it reaches takes a scope`
var (
	Exported = 1
	//declscope:private
	dNarrowed = 2
)

var _ = dNarrowed
