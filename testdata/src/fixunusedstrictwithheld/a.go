package fixdirectivestrictwithheld

// aBare restates the default, and nothing withholds its fix.
//
//declscope:private // want `unused //declscope:private on aBare: it already has private scope`
func aBare() int { return 1 }

// An ignore answers the block's report. The block is redundant, so it stays
// redundant and answered once aSpec takes it, and aSpec's directive goes.
//
//declscope:private
//declscope:ignore directive
var (
	//declscope:private // want `unused //declscope:private on aSpec: it already has private scope`
	aSpec = 2
)

var _ = aBare() + aSpec
