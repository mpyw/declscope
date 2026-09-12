package defaultsmembers

func orderRun(u *User) string {
	u.Bump()
	_ = Helper()
	return u.Name
}

var _ = orderRun
