// Package r exercises the rename guards that the other modules do not.
package r

import _ "unsafe"

func Type() {} // want: func Type is exported.*no fix: the unexported name is not an identifier

func Init() {} // want: func Init is exported.*no fix: the unexported name means something to the toolchain

//go:linkname Pushed
func Pushed() {} // want: func Pushed is exported.*no fix: a //go:linkname or //export names it

// ID and Id lower to one name, so only the first is fixed.
type Pair struct { // want: type Pair is exported, but nothing.*uses it$
	ID int // want: field ID is exported, but nothing.*uses it$
	Id int // want: field Id is exported.*no fix: the unexported name is taken on its type
}

var _ = Pair{}.ID + Pair{}.Id

// Point is written unkeyed by package w through a pointer, and its fields are
// counted as used.
type Point struct{ X, Y int }

// Left is converted to Right through pointers, which pairs X by name.
type Left struct{ X int } // want: type Left is exported, but nothing.*uses it$

type Right struct{ X int } // want: type Right is exported, but nothing.*uses it$

var _ = (*Right)(&Left{})

// Carrier escapes through fmt, and its field of function type hands out a
// Result, whose field reflection can reach by calling it.
type Carrier struct { // want: type Carrier is exported.*no fix: its type escapes
	Make func() Result // want: field Make is exported.*no fix: its type escapes
}

type Result struct { // want: type Result is exported.*no fix: its type escapes
	Value int // want: field Value is exported.*no fix: its type escapes
}

func print(v any) {}

func carry() { print(Carrier{}) }

var _ = carry

// Iface is handed out by pub, and what its method returns reaches another
// module: Out's method and field are used there without naming Out.
type Out struct {
	Field int
}

func (Out) Method() {}

// Ext is written unkeyed by an external test.
type Ext struct { // want: type Ext is exported, but only the external tests.*no fix: external tests use it
	A int // want: field A is exported, but only the external tests.*no fix: external tests use it
}

const Limit = 3 // want: const Limit is exported, but nothing.*uses it$

type Alias = Pair // want: type Alias is exported, but nothing.*uses it$

var _ Alias

// Converting a type to itself or to an unnamed struct: the first pairs
// nothing, the second pairs Solo's field with the unnamed struct's.
type Solo struct { // want: type Solo is exported, but nothing.*uses it$
	X int
}

var _ = Solo(Solo{})
var _ = float64(Limit)
var _ = []int{1}
var _ = struct{ X int }(Solo{})

// An unkeyed literal in the package itself is renamed with the fields.
type Local struct { // want: type Local is exported, but nothing.*uses it$
	A int // want: field A is exported, but nothing.*uses it$
}

var _ = Local{1}

// Bag escapes, and reaches Elem through every kind of container.
type Bag struct { // want: type Bag is exported.*no fix: its type escapes
	S []Elem        // want: field S is exported.*no fix: its type escapes
	R [1]Elem       // want: field R is exported.*no fix: its type escapes
	C chan Elem     // want: field C is exported.*no fix: its type escapes
	M map[Elem]Elem // want: field M is exported.*no fix: its type escapes
	G Gen[Elem]     // want: field G is exported.*no fix: its type escapes
}

type Elem struct{} // want: type Elem is exported.*no fix: its type escapes

type Gen[T any] struct{ v T } // want: type Gen is exported.*no fix: its type escapes

func bag() { print(Bag{}) }

var _ = bag

// Kept is handed out by pub, but its unexported field and method are not
// walked: another module cannot name them.
type Kept struct {
	hidden int
}

func (Kept) secret() {}

var _ = Kept{}.hidden
var _ = Kept{}.secret

// Embedded through a pointer and through an alias, and selected by name.
type Pointed struct{} // want: type Pointed is exported, but nothing.*uses it$

type Aliased struct{} // want: type Aliased is exported, but nothing.*uses it$

type AliasOfAliased = Aliased // want: type AliasOfAliased is exported, but nothing.*uses it$

type holder struct {
	*Pointed
	AliasOfAliased
}

var _ = holder{}.Pointed
var _ = holder{}.AliasOfAliased

// The unexported name is written by a build-excluded file of this package.
func Clash() {} // want: func Clash is exported.*no fix: a build-excluded file of its package writes the unexported name

// The unexported name is an import of another file of the package.
func Strings() {} // want: func Strings is exported.*no fix: the unexported name is taken or would be captured

// An interface of the package asks for the unexported name.
type Quiet struct{} // want: type Quiet is exported, but nothing.*uses it$

func (Quiet) Hush() {} // want: method Hush is exported.*no fix: the unexported name is taken on its type

type husher interface{ hush() }

var _ husher
var _ = Quiet{}.Hush

// Derived promotes Hop, and already has a field of the unexported name.
type Base struct{} // want: type Base is exported, but nothing.*uses it$

func (Base) Hop() {} // want: method Hop is exported.*no fix: the unexported name is taken on its type

type derived struct {
	Base
	hop int
}

var _ = derived{}.Hop
var _ = derived{}.hop

// Pulled is linked by name from package w, as a method.
// The directive spells Linked too, so the type keeps its name.
type Linked struct{}

func (Linked) Pulled() {}

// An ignore bound to no declaration silences nothing, and nothing beside it
// answers its unused report.
func stray() {
	//declscope:ignore overexported // want: unused //declscope:ignore overexported
}

var _ = stray

// A predeclared type embedded by name is no declaration of the package.
type basic struct{ int }

var _ = basic{}.int
