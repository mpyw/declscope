package z

// T reaches another module through y.Get, so M and F are used there.
type T struct {
	F int
}

func (T) M() {}

func Unused() {} // want: func Unused is exported, but nothing.*uses it$
