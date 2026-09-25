// Package m exercises the rename of a method or field.
package m

import _ "unsafe"

// Named has a method whose unexported name is a field, and a field whose
// unexported name is a method.
type Named struct { // want: type Named is exported, but nothing.*uses it$
	name string
	Size int // want: field Size is exported.*no fix: the unexported name is taken on its type
}

func (Named) Name() string { return "" } // want: method Name is exported.*no fix: the unexported name is taken on its type

func (Named) size() int { return 0 }

// Asked would start satisfying the interface below once Run became run.
type Asked struct{} // want: type Asked is exported, but nothing.*uses it$

func (Asked) Run() {} // want: method Run is exported.*no fix: the unexported name is taken on its type

var _ = func(v any) bool { _, ok := v.(interface{ run() }); return ok }

// Inner is embedded, so s.Inner selects the field by the type's name, and
// the rename rewrites the selection as well.
type Inner struct{} // want: type Inner is exported, but nothing.*uses it$

type outer struct{ Inner }

var _ = outer{}.Inner

var _ = Named{}.name
var _ = Named{}.Size + Named{}.size()
var _ = Named{}.Name
var _ = Asked{}.Run

// Linked is pulled by a //go:linkname in package u, which names it as text.
func Linked() int { return 1 }

// Point is written unkeyed by package u, which can only while its fields are
// exported.
type Point struct{ X, Y int }

// Left and Right are converted into each other, which pairs X by name.
type Left struct { // want: type Left is exported, but nothing.*uses it$
	X int
}

type Right struct { // want: type Right is exported, but nothing.*uses it$
	X int
}

var _ = Right(Left{})

// Dotted is named under a dot import by a build-excluded file of package u.
var Dotted = 1 // want: var Dotted is exported.*no fix: a build-excluded file of another package may use it

// Picker has a method a build-excluded file of package u selects by name.
type Picker struct{} // want: type Picker is exported, but nothing.*uses it$

func (Picker) Pick() {} // want: method Pick is exported.*no fix: a build-excluded file of another package may use it

var _ = Picker{}.Pick

// Promoted is embedded by pub.Wrapper, which pub hands out.
type Promoted struct {
	Via int
}

func (Promoted) Call() {}
