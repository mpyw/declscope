package fixreflect

import (
	"encoding/gob"
	"encoding/xml"
	"fmt"
)

// point is marshalled to XML, which names the element after it.
type point struct{ X int } // want `type point does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userPoint is one candidate, not necessarily the best\)`

// shape is printed with %T, which prints its name.
type shape struct{ X int } // want `type shape does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userShape is one candidate, not necessarily the best\)`

// token is registered with gob, which records its name.
type token struct{ X int } // want `type token does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userToken is one candidate, not necessarily the best\)`

// cell reaches an interface inside a slice.
type cell struct{ X int } // want `type cell does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userCell is one candidate, not necessarily the best\)`

// inner reaches an interface embedded in userPair, which %+v prints by
// the field's name.
type inner struct{ X int } // want `type inner does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userInner is one candidate, not necessarily the best\)`

// boxed reaches an interface through a pointer, by an implicit
// conversion at an assignment.
type boxed struct{ X int } // want `type boxed does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userBoxed is one candidate, not necessarily the best\)`

// result is converted to any where it is returned.
type result struct{ X int } // want `type result does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userResult is one candidate, not necessarily the best\)`

// sent is sent on a channel of any.
type sent struct{ X int } // want `type sent does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userSent is one candidate, not necessarily the best\)`

// generic is a type argument of a function that prints it.
type generic struct{ X int } // want `type generic does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userGeneric is one candidate, not necessarily the best\)`

// tally never reaches an interface, so it is renamed.
type tally struct{ n int } // want `type tally does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userTally is one candidate, not necessarily the best\)`

type userPair struct {
	inner
	Y int
}

func userRender() string {
	b, _ := xml.Marshal(point{X: 1})
	gob.Register(token{})
	var v any = &boxed{}
	return fmt.Sprintf("%s %T %v %+v %v", b, shape{}, []cell{{}}, userPair{}, v)
}

func userResult() any { return result{} }

func userSend(ch chan any) { ch <- sent{} }

func userShow[T any](v T) string { return fmt.Sprint(v) }

func userGeneric() string { return userShow(generic{}) }

func userCount() int { return tally{n: 1}.n }

// exposed is returned by an exported function, and an importer can convert
// it to an interface where this pass cannot see.
type exposed struct{ X int } // want `type exposed does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userExposed is one candidate, not necessarily the best\)`

func UserExposed() exposed { return exposed{} }
