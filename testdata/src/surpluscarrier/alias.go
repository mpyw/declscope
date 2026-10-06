package surpluscarrier

type impl struct{}

//declscope:shared
func (impl) fire() {}

// Public is an exported alias: the method set travels under its name.
type Public = impl
