//declscope:namespace user
package directives

// This file joins namespace "user", so it may use its file-private
// declarations without promoting them.
func reportAll() int {
	return explicitlyShared() + userNotReallyShared() + ignoredLeak() + trailing() + sharedA + privateB
}

var _ = reportAll
var _ = neverLeaks
var _ = userTypo
