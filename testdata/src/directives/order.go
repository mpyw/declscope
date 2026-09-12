package directives

func orderRun() int {
	return explicitlyShared() + userNotReallyShared() + ignoredLeak() + trailing() + sharedA + privateB
}

var _ = orderRun
