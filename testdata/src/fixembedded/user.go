package fixembedded

type count struct{ n int } // want `type count does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userCount, or move it to the file it belongs to`

type total struct{ sum int } // want `type total does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userTotal, or move it to the file it belongs to`

type userBase struct{ v int }

// view is an alias. A field embedding it is spelled with the alias's name, not
// the type it denotes, so the rename rewrites the selection through it too.
type view = userBase // want `type view does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userView, or move it to the file it belongs to`

// User embeds all three. The embedding ident and every selection through it are
// spelled with the type's name, so the rename has to rewrite all of them.
type User struct {
	count
	*total
	view
}

func (u *User) Sum() int { return u.count.n + u.total.sum + u.view.v }
