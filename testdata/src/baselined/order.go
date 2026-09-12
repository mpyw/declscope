package baselined

func orderRun(u *User) string {
	_ = userHelper() + userFresh()
	return u.name + u.note
}

var _ = orderRun
