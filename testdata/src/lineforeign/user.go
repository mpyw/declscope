package lineforeign

// The type below a //line directive is still declared in user.go, so a method
// filed in order.go is away from it.
//
//line user.tmpl:1
type userRecord struct { // want `type userRecord is private to namespace "user", but is used from namespace "order"`
	name string // want `field userRecord.name is private to namespace "user", but is used from namespace "order"`
}
