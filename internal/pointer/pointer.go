// Package pointer finds what a pointer type points to, in every form Go
// writes one. The analyzer and declscope shrink both ask it, where a
// composite literal leaves out a pointer type and where a conversion pairs
// the fields of two structs behind pointers.
package pointer

import "go/types"

// Elem returns the type t points to, and whether t is a pointer at all.
//
// A pointer is not only *T. A defined type such as type PSlot *Slot is one
// through its underlying type, and so is a type parameter whose constraint
// allows only pointers to one type, such as P interface{ ~*Slot }: every
// type in its type set has the same underlying pointer, which is what Go
// calls its core type. []PSlot{{1}} and []P{{1}} write the fields of Slot,
// and P(u) converts u to a pointer to Slot. Reading *types.Pointer alone
// missed all three.
func Elem(t types.Type) (types.Type, bool) {
	var u types.Type
	if tp, ok := types.Unalias(t).(*types.TypeParam); ok {
		u = core(tp)
	} else {
		u = t.Underlying()
	}
	if ptr, ok := u.(*types.Pointer); ok {
		return ptr.Elem(), true
	}
	return nil, false
}

// core returns the underlying type every type in the type set of tp shares,
// or nil when the types differ or the set is not limited by its terms.
//
// Terms are collected from every embedded element of the constraint,
// through the interfaces it embeds. Several elements intersect, so their
// terms agreeing on one underlying type is enough. Terms that disagree may
// still intersect in one type, as in interface{ ~*A | ~*B; ~*A }. That is
// read as no core type, which only misses a use.
func core(tp *types.TypeParam) types.Type {
	var found types.Type
	agree := true
	add := func(u types.Type) {
		switch {
		case found == nil:
			found = u
		case !types.Identical(found, u):
			agree = false
		}
	}
	var walk func(iface *types.Interface)
	walk = func(iface *types.Interface) {
		for e := range iface.EmbeddedTypes() {
			if union, ok := e.(*types.Union); ok {
				for i := range union.Len() {
					u := union.Term(i).Type().Underlying()
					// A term that is itself an interface lets in every type
					// it allows, which no term here limits.
					if _, isInterface := u.(*types.Interface); isInterface {
						agree = false
					}
					add(u)
				}
				continue
			}
			if inner, ok := e.Underlying().(*types.Interface); ok {
				walk(inner)
				continue
			}
			add(e.Underlying())
		}
	}
	if iface, ok := tp.Constraint().Underlying().(*types.Interface); ok {
		walk(iface)
	}
	if !agree {
		return nil
	}
	return found
}
