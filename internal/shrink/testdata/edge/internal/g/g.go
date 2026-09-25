// Package g exercises generics: a selection on an instantiation names the
// origin's member, and a conversion inside an instantiated generic function
// is seen with the concrete type.
package g

import (
	"fmt"
	"sync/atomic"
)

type List[T any] struct { // want: type List is exported, but nothing.*uses it$
	Items []T // want: field Items is exported, but nothing.*uses it$
}

func (l *List[T]) Len() int { return len(l.Items) } // want: method Len is exported.*no fix: the unexported name is predeclared

func count() int {
	var l List[int]
	return l.Len() + len(l.Items)
}

var _ = count

// Shown reaches fmt.Println through a generic function of the module.
type Shown struct{} // want: type Shown is exported.*no fix: its type escapes

func show[T any](v T) { fmt.Println(v) }

// useShown is unused, but it is a function of the package, so its body is
// built and the conversion inside show[Shown] is seen.
func useShown() { show(Shown{}) }

// Held is a type argument of a generic declared outside the module, whose
// body is not built here, so it is taken to escape.
type Held struct{} // want: type Held is exported.*no fix: its type escapes

var _ atomic.Pointer[Held]
