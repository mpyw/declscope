package foreign

// normalize reaches into another namespace's type from the outside.
func (u *User) normalize() { u.ID++ } // want `unexported method User.normalize is declared in namespace "order" but User belongs to namespace "user"`

// Normalize is exported, so it is public regardless of where it is declared.
func (u *User) Normalize() { u.ID++ }
