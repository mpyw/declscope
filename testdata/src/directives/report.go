//declscope:namespace user

package directives

// This file joins namespace "user", so it may use its namespace-private
// declarations without widening them, and its own declarations carry the
// "user" prefix rather than a "report" one.
func userReportAll() int {
	return userExplicitlyShared() + userNotReallyShared() + userIgnoredLeak() +
		userTrailing() + userSharedA + userPrivateB + userSharedC + userPrivateD
}

var _ = userReportAll
var _ = userNeverLeaks
var _ = userTypo
