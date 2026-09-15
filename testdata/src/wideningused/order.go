package wideningused

func orderRun() int {
	return userShared() + userMakePair().second
}

var (
	_ = orderRun
	_ = userMake
	_ = UserRec{7}
)
