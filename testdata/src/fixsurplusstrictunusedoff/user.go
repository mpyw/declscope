//declscope:package

package fixsurplusstrictunusedoff

// userHelper takes the file's package, and order.go calls it.
func userHelper() int { return 3 }

// userSpare takes it too, and nothing else calls it. Its narrowing gives
// this file an edit whatever the block's specs get, so the golden is compared.
func userSpare() int { return 4 } // want `func userSpare takes package scope`

var _ = userSpare

// The block restates the file's package and decides nothing. With the unused
// rule off nothing reports it, so userLocal is narrowed.
//
//declscope:package
var (
	userShared = 1
	userLocal  = 2 // want `var userLocal takes package scope`
)

var _ = userLocal
