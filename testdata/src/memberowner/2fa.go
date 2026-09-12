package memberowner

// The file name yields no namespace, so Auth is described by its file. Its
// members are bounded by this file whichever file they are written in.
type Auth struct {
	secret int // want `field Auth.secret is private to file 2fa.go, but is used from namespace "order"`
}
