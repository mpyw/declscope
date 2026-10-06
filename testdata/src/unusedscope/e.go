//declscope:private // want `unused file-level //declscope:private$`

package unusedscope

// The same under a file-level directive: eOther restates the file, and
// eWidened overrides the block.
//
//declscope:private // want `unused //declscope:private on eOther: nothing it reaches takes a scope`
var (
	eOther = 1
	//declscope:shared
	eWidened = 2
)

var _ = eOther + eWidened
