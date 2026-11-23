//declscope:shared

package fixsurplusstrictunusedoff

// userHelper takes the file's package, and order.go calls it.
func userHelper() int { return 3 }

// userSpare takes it too, and nothing else calls it. Its narrowing gives
// this file an edit whatever the block's specs get, so the golden is compared.
func userSpare() int { return 4 } // want `func userSpare takes shared scope`

var _ = userSpare

// The block restates the file's package and decides nothing. With the unused
// rule off nothing reports it, so userLocal is narrowed.
//
//declscope:shared
var (
	userShared = 1
	userLocal  = 2 // want `var userLocal takes shared scope`
)

var _ = userLocal

// userBox takes the file's package too, and nothing else names it. Its field
// states //declscope:private, which binds only while the type is shared. With
// the unused rule off nothing reports that directive once the type is
// narrowed, so the fix is offered.
type userBox struct { // want `type userBox takes shared scope`
	//declscope:private
	count int
}

var _ = userBox{}.count
