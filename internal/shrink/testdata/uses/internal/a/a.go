// Package a is judged. Each exported declaration here is used from outside the
// package in one way, and is not reported, unless a want comment says so.
package a

import (
	"fmt"
	"io"
)

// Spelled is named by package b.
func Spelled() {}

// Point is written unkeyed by package b, through a pointer, which b can only
// while every field is exported.
type Point struct{ X, Y int }

// Left and Right are converted into each other, which pairs X by name. The
// type names themselves are free to go.
type Left struct { // want: type Left is exported, but nothing.*uses it$
	X int
}

type Right struct { // want: type Right is exported, but nothing.*uses it$
	X int
}

var (
	_ = Right(Left{})
	_ = (*Right)(&Left{})
)

// Solo is converted to itself, which pairs nothing, and to an unnamed struct,
// which pairs X with the unnamed struct's field.
type Solo struct { // want: type Solo is exported, but nothing.*uses it$
	X int
}

var (
	_ = Solo(Solo{})
	_ = struct{ X int }(Solo{})
)

// Linked is pulled by a //go:linkname in package b.
func Linked() int { return 1 }

// ByValue and ByPointer have methods pulled as T.M and (*T).M, which spell
// the type as well.
type ByValue struct{}

func (ByValue) Pulled() {}

type ByPointer struct{}

func (*ByPointer) Pulled() {}

// Wr satisfies io.Writer where the compiler checks it, so Write is used.
type Wr struct{} // want: type Wr is exported, but nothing.*uses it$

func (Wr) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = Wr{}

// Label's String satisfies the constraint of show, instantiated with Label.
type Label struct{} // want: type Label is exported, but nothing.*uses it$

func (Label) String() string { return "" }

func show[T fmt.Stringer](v T) string { return v.String() }

var _ = show[Label]

// Exposed is handed out by pub.Get, so another module can call M and read F
// on a value of it without naming the type.
type Exposed struct {
	F int
}

func (Exposed) M() {}

// Promoted is embedded by pub.Wrapper, which pub hands out.
type Promoted struct {
	Via int
}

func (Promoted) Call() {}

// Inner is embedded in Outer, which pub hands out, so another module selects
// v.Inner by the type's name.
type Inner struct{}

type Outer struct{ Inner }

// Out is what the method of pub.Iface returns.
type Out struct {
	Field int
}

func (Out) Method() {}

// Kept is held by pub.K, but its unexported field and method are no part of
// what another module can reach.
type Kept struct {
	hidden int
}

func (Kept) secret() {}

var (
	_ = Kept{}.hidden
	_ = Kept{}.secret
)

// Made is returned by Make, which package b calls. b holds a Made without
// naming it, and must still be able to name it.
type Made struct {
	V int // want: field V is exported, but nothing.*uses it$
}

func Make() Made { return Made{} }

// ExclQual is named only by a build-excluded file of package b.
var ExclQual = 1

// OnlyExtTest, Ext and Shown are used by the external tests alone.
var OnlyExtTest = 1 // want: var OnlyExtTest is exported, but only the external tests.*no fix: external tests use it

type Ext struct { // want: type Ext is exported, but only the external tests.*no fix: external tests use it
	A int // want: field A is exported, but only the external tests.*no fix: external tests use it
}

func Shown() {} // want: func Shown is exported, but only the external tests.*no fix: external tests use it

func parse() int { return 2 }
