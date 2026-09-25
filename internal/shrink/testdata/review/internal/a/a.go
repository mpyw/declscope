// Package a holds one case per finding of the review of the first version.
package a

// Inner is embedded in Outer, which pub hands out, so another module can
// select v.Inner by the type's name. Nothing is said about it.
type Inner struct{}

type Outer struct{ Inner }

// Emb's unexported name is taken by a field of a struct embedding it: the
// embedded field would take that name too.
type Emb struct{} // want: type Emb is exported.*no fix: the unexported name is taken or would be captured

type Holder struct { // want: type Holder is exported, but nothing.*uses it$
	Emb
	emb int
}

var _ = Holder{}.emb
var _ = Holder{}.Emb

// P's method is pulled by a //go:linkname on a pointer receiver, which spells
// the type as well.
type P struct{}

func (*P) M() {}

// A trailing ignore on a func's first or last line is read, as the analyzer
// reads it.
func Trailing() {} //declscope:ignore overexported

func Multi() {
} //declscope:ignore overexported

// A silenced declaration claims no name, so FOO's fix is offered.
//
//declscope:ignore overexported
func Foo() {}

func FOO() {} // want: func FOO is exported, but nothing.*uses it$

// Level is not a struct, so it embeds nothing to collide with.
type Level int // want: type Level is exported, but nothing.*uses it$

var _ Level
