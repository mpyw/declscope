package wideningcarrier

type impl struct{}

//declscope:package
func (impl) fire() {}

// Public is an exported alias: the method set travels under its name.
type Public = impl
