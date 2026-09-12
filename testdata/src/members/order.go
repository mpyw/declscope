package members

func orderDump(u *User) string {
	u.normalize()
	return u.name
}

var _ = orderDump
var _ = userMake
