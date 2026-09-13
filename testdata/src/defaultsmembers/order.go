package defaultsmembers

func orderRun(u *User) string {
	u.bump()
	u.seal()
	_ = u.name
	return u.secret
}

var _ = orderRun
