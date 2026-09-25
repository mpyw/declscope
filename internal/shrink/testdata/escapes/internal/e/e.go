// Package e holds values that escape into an interface, where fmt,
// encoding/json and reflect find methods and fields at run time. An escape is
// a use, so none of them is reported. So is a struct tag, which declares one.
package e

import (
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
)

// Printed escapes through fmt.Println, behind a pointer.
type Printed struct {
	Field int
}

func (Printed) Method() {}

func printed() { fmt.Println(&Printed{}) }

// Wire shares Record's struct, so json.Marshal(Wire(r)) reads Record's
// fields under either name. Record's own name is not read, and is reported.
type Record struct { // want: type Record is exported, but nothing.*uses it$
	Name string
}

type Wire Record

func encode(r Record) ([]byte, error) { return json.Marshal(Wire(r)) }

// Shown reaches fmt.Println inside show[Shown], a generic of the module that
// is instantiated with it.
type Shown struct{}

func show[T any](v T) { fmt.Println(v) }

func useShown() { show(Shown{}) }

// Held is a type argument of sync/atomic.Pointer, a generic declared outside
// the module whose body is not built here, so it is taken to escape.
type Held struct{}

var _ atomic.Pointer[Held]

// Bag escapes, and reaches Elem through every kind of container and through
// an instantiation.
type Bag struct {
	S []Elem
	R [1]Elem
	C chan Elem
	M map[Elem]Elem
	G Gen[Elem]
}

type Elem struct{}

type Gen[T any] struct{ v T }

func bag() { fmt.Println(Bag{}) }

// Carrier escapes, and its field of function type hands out a Result, whose
// field reflection reaches by calling it.
type Carrier struct {
	Make func() Result
}

type Result struct {
	Value int
}

func carry() { fmt.Println(Carrier{}) }

// Wr satisfies io.Writer only in a check that makes no value at run time,
// which is no escape.
type Wr struct{} // want: type Wr is exported, but nothing.*uses it$

func (Wr) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = Wr{}

// Tagged is never converted to an interface here, but its struct tags say
// reflection reads its fields, and go vet rejects a json tag on an unexported
// field. The tags are a use, so the fields are not reported.
type Tagged struct { // want: type Tagged is exported, but nothing.*uses it$
	Name  string `json:"name"`
	Count int    `xml:"count"`
	Plain int // want: field Plain is exported, but nothing.*uses it$
}

var _ = Tagged{}.Name + fmt.Sprint(Tagged{}.Count, Tagged{}.Plain)

// Dead is converted only in a function literal that never runs, which is no
// escape either.
type Dead struct{} // want: type Dead is exported, but nothing.*uses it$

var _ = func() { fmt.Println(Dead{}) }

var _, _, _, _, _, _ = printed, encode, useShown, bag, carry, Gen[int]{}.v
