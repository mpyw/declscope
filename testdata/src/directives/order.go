package directives

func orderRun() int {
	return userExplicitlyShared() + userNotReallyShared() + userIgnoredLeak() +
		userTrailing() + userSharedA + userPrivateB + userSharedC + userPrivateD
}

var _ = orderRun
