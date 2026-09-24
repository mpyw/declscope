package fixdirectivestrictwithheld

// The block keeps its report, since e.go uses dA. Deleting dB's directive
// would hand dB to the block, and the block's report would name it too.
//
//declscope:private // want `unused //declscope:private on dA: it already has private scope`
var (
	dA = 1 // want `var dA is declared private by //declscope:private, but is used from namespace "e"`
	//declscope:private // want `unused //declscope:private on dB: it already has private scope`
	dB = 2
)

var _ = dB
