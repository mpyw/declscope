package memberowner

// Auth's members are bounded by the namespace of this file, whichever file
// they are written in.
type Auth struct {
	secret int // want `field Auth.secret is private to namespace "2fa", but is used from namespace "order"`
}
