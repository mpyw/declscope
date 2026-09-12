package pkglevel

func orderRun() int {
	_ = userPayload{}
	return userHelper() + userShared() + userCount + userLimit + userTotal
}

var _ = orderRun
