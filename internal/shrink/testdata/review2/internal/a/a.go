// Package a holds one case per finding of the second review.
package a

import "encoding/json"

// Wire shares Record's struct, so Wire escaping into json.Marshal exposes
// Record's fields to reflection under either name. Record's own name is not
// read by it.
type Record struct { // want: type Record is exported, but nothing.*uses it$
	Name string // want: field Name is exported.*no fix: its type escapes
}

type Wire Record // want: type Wire is exported.*no fix: its type escapes

func encode(r Record) ([]byte, error) { return json.Marshal(Wire(r)) }

var _ = encode

// An unnamed struct promotes Size, and has a field of the unexported name.
type Named struct { // want: type Named is exported, but nothing.*uses it$
	Size int // want: field Size is exported.*no fix: the unexported name is taken on its type
}

func useNamed() int {
	v := struct {
		Named
		size int
	}{}
	return v.Size + v.size
}

var _ = useNamed

// ID and Id belong to different types, and Both embeds the two: renaming
// both to id would make Both{}.id ambiguous, so only the first is fixed.
type First struct { // want: type First is exported, but nothing.*uses it$
	ID int // want: field ID is exported, but nothing.*uses it$
}

type Second struct { // want: type Second is exported, but nothing.*uses it$
	Id int // want: field Id is exported.*no fix: the unexported name is taken on its type
}

type both struct {
	First
	Second
}

var _ = both{}.ID + both{}.Id

// Plug is held by a variable of package main, which a plugin host looks up,
// so its method and field are used there.
type Plug struct {
	F int
}

func (Plug) M() {}

// An ignore answered as the analyzer answers one: an ignore beside it naming
// unused keeps its unused report quiet.
//
//declscope:ignore overexported
//declscope:ignore unused
func Answered() int { return Used() }

func Used() int { return 1 } // want: func Used is exported, but nothing.*uses it$
