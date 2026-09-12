package directives

func orderRun() int {
	return userExplicitlyShared() + userNotReallyShared() + userIgnoredLeak() +
		userTrailing() + userSharedA + userPrivateB
}

var _ = orderRun
