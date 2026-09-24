//declscope:package

package unusedstrict

// The block restates the file's package, so strict reports it. Its only spec
// states private, so the report says the spec states its own scope: it does
// not have the block's.
//
//declscope:package // want `unused //declscope:package: every declaration it reaches states its own scope`
var (
	//declscope:private
	hNarrowed = 1
)

func hWide() int { return 2 }

var _ = hNarrowed + hWide()
