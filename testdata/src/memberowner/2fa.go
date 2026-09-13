package memberowner

// Auth's fields are written inside this declaration, so they are bounded by
// this file's namespace wherever they are read.
type Auth struct {
	secret int // want `field Auth.secret is private to namespace "2fa", but is used from namespace "order"`
}
