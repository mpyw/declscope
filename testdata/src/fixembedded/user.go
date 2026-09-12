package fixembedded

type count struct{ n int } // want `type count does not carry the prefix of namespace "user"; rename it to userCount`

type total struct{ sum int } // want `type total does not carry the prefix of namespace "user"; rename it to userTotal`

// User embeds both. The embedding ident and every selection through it are
// spelled with the type's name, so the rename has to rewrite all of them.
type User struct {
	count
	*total
}

func (u *User) Sum() int { return u.count.n + u.total.sum }
