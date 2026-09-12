package pkglevel

// The prefix is an ownership label and grants nothing, so this is still
// private to its namespace.
func userHelper() int { return 1 } // want `func userHelper is file-private to namespace "user", but is used from namespace "order"`

// Widening is always an explicit act.
//
//declscope:package
func userShared() int { return 2 }

// Exported identifiers are public, and carry no prefix.
func Exported() int { return userHelper() }

var userCount int // want `var userCount is file-private to namespace "user", but is used from namespace "order"`

//declscope:package
var userTotal int

const userLimit = 10 // want `const userLimit is file-private to namespace "user", but is used from namespace "order"`

type userPayload struct{} // want `type userPayload is file-private to namespace "user", but is used from namespace "order"`

// Used only inside its own namespace, and none the worse for carrying the
// prefix: the label says which unit owns it, not how far it reaches.
func userLocal() int { return userCount }

var _ = userLocal
