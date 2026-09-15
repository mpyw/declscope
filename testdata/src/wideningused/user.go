package wideningused

// Spelled from order.go, so the directive is doing its job.
//
//declscope:package
func userShared() int { return 1 }

// One field's use keeps the whole comment: deleting it would narrow second
// back, which order.go still reaches.
//
//declscope:package
type userPair struct {
	first  int
	second int
}

//declscope:package
func userMakePair() userPair { return userPair{} }

// The field narrows itself back, so it does not depend on the type's
// directive, and the type's comment is judged on the type alone.
//
//declscope:package // want `//declscope:package on userSolo: no use from another namespace is visible to declscope`
type userSolo struct {
	//declscope:private
	name string
}

//declscope:package
func userMake() userSolo { return userSolo{name: "n"} }

// UserRec is exported, so spelling it from order.go keeps nothing here alive.
// The field's directive is kept only by the composite literal without keys,
// which writes the field without spelling it and still counts as the use it
// is.
type UserRec struct {
	//declscope:package
	n int
}
