package foreign

// Never called, so no cross-namespace reference exists to find. This is the
// gap the declaration-site rule covers.
func (u *User) normalize() { u.ID++ } // want `unexported method User.normalize is declared in namespace "order" but User belongs to namespace "user"`

// Called from here, so the boundary crossing rule already reports it at this
// very position. Reporting both would say the same thing twice.
func (u *User) bump() { u.ID += 2 } // want `method User.bump is private to namespace "user", but is used from namespace "order"`

// Exported, so it is public regardless of where it is declared.
func (u *User) Normalize() { u.bump() }
