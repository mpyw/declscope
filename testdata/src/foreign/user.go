package foreign

type User struct {
	ID int
}

func (u *User) reset() { u.ID = 0 }
