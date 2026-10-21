package surplusused

func orderRun() int {
	return userShared() + userMakePair().second
}

var (
	_ = orderRun
	_ = userMake
	_ = UserRec{7}
	_ = append(userSlots(), []userSlotPtr{{1}}...)
)
