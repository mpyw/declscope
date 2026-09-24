//declscope:package

package fixsurplusstrict

// restatedHelper takes the file's package, and order.go calls it.
func restatedHelper() int { return 3 }

// The block restates the file's package, so it decides nothing, and the
// unused rule reports it naming both specs. Narrowing restatedLocal would
// reword that report, so the fix is withheld and this file takes no edit. The
// block is the thing to delete, which no fix does.
//
//declscope:package // want `unused //declscope:package on restatedShared, restatedLocal: nothing it reaches takes a scope`
var (
	restatedShared = 1
	restatedLocal  = 2 // want `var restatedLocal takes package scope`
)

var _ = restatedLocal
