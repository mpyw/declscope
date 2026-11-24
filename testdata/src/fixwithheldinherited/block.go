package fixwithheldinherited

// A spec takes private from its block alone, and so does a member of a type
// declared in a block.
//
//declscope:private
var (
	blockSeed = 1 // want `var blockSeed is declared private by //declscope:private, but is used from namespace "order"`
)

//declscope:private
type (
	blockBox struct {
		n int // want `field blockBox.n is declared private by //declscope:private on blockBox, but is used from namespace "order"`
	}
)

func BlockMake() blockBox { return blockBox{} }
