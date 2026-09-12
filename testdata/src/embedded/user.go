package embedded

// userCount is embedded by another namespace. The embedding names the type,
// so it is a use of the type like any other.
type userCount struct { // want `type userCount is file-private to namespace "user", but is used from namespace "order"`
	n int // want `field userCount.n is private to namespace "user", but is used from namespace "order"`
}

// userTotal is embedded by pointer.
type userTotal struct { // want `type userTotal is file-private to namespace "user", but is used from namespace "order"`
	sum int
}

// User embeds both inside their own namespace, which crosses nothing.
type User struct {
	userCount
	*userTotal
}

func (u *User) Sum() int { return u.n + u.sum }
