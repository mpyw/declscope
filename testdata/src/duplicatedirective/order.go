package duplicatedirective

func orderRun() int {
	return sharedHelper() + userShared() + UserPrivate() + userIgnored() + userTrailing() + userWider()
}

var _ = orderRun
