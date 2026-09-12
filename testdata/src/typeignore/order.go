package typeignore

func orderRun(u *User, o *Open, c *Closed) string {
	u.normalize()
	return u.name + string(rune(u.id)) + o.secret + c.hidden
}

var _ = orderRun
