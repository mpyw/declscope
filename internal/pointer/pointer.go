// Package pointer finds the structs whose fields a composite literal or a
// conversion writes without naming them, through a pointer in every form Go
// writes one and through the type set of a type parameter. The analyzer and
// declscope shrink both ask it, where an unkeyed literal fills fields by
// position and where a conversion pairs the fields of two structs by name.
package pointer

import (
	"go/types"
	"slices"

	"github.com/mpyw/declscope/internal/typeparams"
)

// Sets holds the type set of each type parameter asked about, for one pass.
// A constraint is a tree of interfaces, and every literal and conversion of a
// type parameter would otherwise walk it again. The set is keyed by the
// constraint, since it reads nothing else of the type parameter, so every
// type parameter constrained by one C shares one computation. The zero value
// is ready.
type Sets struct {
	byConstraint map[types.Type]paramSet
}

// paramSet is the type set of one type parameter. When it could not be
// computed, terms holds every term the constraint writes instead: a superset
// of the type set, since normalizing only joins and intersects terms.
type paramSet struct {
	terms []*types.Term
	known bool
}

// Literal returns the types whose fields an unkeyed composite literal of type
// t writes, by position, and whether that is exact. Each has a struct as its
// underlying type.
//
// The literal's type is not only a struct type. A literal inside another may
// leave out a pointer type, so the inner literal of []*Slot{{1}} has the type
// *Slot. The pointer may be a defined type, type PSlot *Slot. And the type may
// be a type parameter: P{1} and []P{{1}} need its type set to have one
// underlying type, its core type, which is a struct or a pointer to one. The
// set may hold several types with that struct as their underlying type, as in
// P interface{ Left | Right }, and the literal writes the fields of each:
// renaming one breaks it.
//
// When the set of a type parameter cannot be computed (an error, or the term
// limit go/types also enforces), the types are every struct the constraint
// writes, through one pointer, and the result is not exact. A caller must then
// read each as a possible use, never the lack of one as no use.
func (s *Sets) Literal(t types.Type) ([]types.Type, bool) {
	tp, isParam := types.Unalias(t).(*types.TypeParam)
	if !isParam {
		u := t.Underlying()
		if ptr, ok := u.(*types.Pointer); ok {
			return s.structCore(ptr.Elem())
		}
		if _, ok := u.(*types.Struct); ok {
			return []types.Type{t}, true
		}
		return nil, true
	}
	set := s.of(tp)
	if !set.known {
		return s.Conversion(t)
	}
	if ptr, ok := typeparams.CoreType(set.terms).(*types.Pointer); ok {
		return s.structCore(ptr.Elem())
	}
	return s.structCore(t)
}

// structCore returns the types of t's type set when their core type is a
// struct, the way Literal reads the type a pointer points to: a literal leaves
// out one pointer, never two.
func (s *Sets) structCore(t types.Type) ([]types.Type, bool) {
	tp, isParam := types.Unalias(t).(*types.TypeParam)
	if !isParam {
		if _, ok := t.Underlying().(*types.Struct); ok {
			return []types.Type{t}, true
		}
		return nil, true
	}
	set := s.of(tp)
	if !set.known {
		return s.Conversion(t)
	}
	if _, ok := typeparams.CoreType(set.terms).(*types.Struct); !ok {
		return nil, true
	}
	out := make([]types.Type, len(set.terms))
	for i, term := range set.terms {
		out[i] = term.Type()
	}
	return out, true
}

// Conversion returns the types whose fields a conversion to or from t pairs
// by name, and whether that is exact. Each has a struct as its underlying
// type.
//
// (*Snap)(&user) converts between pointers to two structs, and pairs their
// fields as Snap(user) does. A conversion to or from a type parameter needs no
// core type: it converts to or from each type in the set, so each struct
// there, or behind a pointer there, is paired. An inexact result is read as
// Literal says.
func (s *Sets) Conversion(t types.Type) ([]types.Type, bool) {
	var out []types.Type
	known := true
	add := func(t types.Type) {
		if _, ok := t.Underlying().(*types.Struct); ok {
			out = append(out, t)
		}
	}
	for _, x := range s.terms(t, &known) {
		ptr, ok := x.Underlying().(*types.Pointer)
		if !ok {
			add(x)
			continue
		}
		for _, elem := range s.terms(ptr.Elem(), &known) {
			add(elem)
		}
	}
	return out, known
}

// terms returns the types of t's type set: t alone, unless it is a type
// parameter. It clears known when the set had to be guessed.
func (s *Sets) terms(t types.Type, known *bool) []types.Type {
	tp, isParam := types.Unalias(t).(*types.TypeParam)
	if !isParam {
		return []types.Type{t}
	}
	set := s.of(tp)
	*known = *known && set.known
	out := make([]types.Type, len(set.terms))
	for i, term := range set.terms {
		out[i] = term.Type()
	}
	return out
}

// of returns the type set of tp, computed once per constraint for a pass.
// NormalTerms and writtenTerms read only the constraint. go/types gives a
// receiver's type parameter the constraint of the type's own when it names no
// type parameter, and a substituted copy when it does, as in ~[]T. It shares
// the instances of a generic constraint, G[int], and keeps G[string] apart.
func (s *Sets) of(tp *types.TypeParam) paramSet {
	constraint := tp.Constraint()
	if set, ok := s.byConstraint[constraint]; ok {
		return set
	}
	if s.byConstraint == nil {
		s.byConstraint = map[types.Type]paramSet{}
	}
	terms, err := typeparams.NormalTerms(tp)
	written := writtenTerms(constraint, map[types.Type]bool{})
	set := paramSet{terms: terms, known: err == nil}
	if err != nil {
		set.terms = written
		s.byConstraint[constraint] = set
		return set
	}
	// A term a tilde term absorbed still counts. The normal form of
	// interface{ ~struct{ X int } | S } is ~struct{ X int } alone, since S has
	// that underlying type. Unexporting a field of S changes it, S is no
	// longer absorbed, and the set loses its core type, so a literal of P
	// breaks. Each such term has the underlying type of one in the set, and
	// so leaves the core type as it is.
	for _, w := range written {
		absorbed := slices.ContainsFunc(set.terms, func(t *types.Term) bool {
			return t.Tilde() && types.Identical(w.Type().Underlying(), t.Type())
		})
		listed := slices.ContainsFunc(set.terms, func(t *types.Term) bool { return types.Identical(w.Type(), t.Type()) })
		if absorbed && !listed {
			set.terms = append(set.terms, w)
		}
	}
	s.byConstraint[constraint] = set
	return set
}

// writtenTerms returns every term a constraint writes, through the interfaces it
// embeds and the interface terms of its unions, left as written.
func writtenTerms(constraint types.Type, seen map[types.Type]bool) []*types.Term {
	iface, ok := constraint.Underlying().(*types.Interface)
	if !ok || seen[constraint] {
		return nil
	}
	seen[constraint] = true
	var out []*types.Term
	for e := range iface.EmbeddedTypes() {
		union, ok := e.(*types.Union)
		if !ok {
			if _, isInterface := e.Underlying().(*types.Interface); isInterface {
				out = append(out, writtenTerms(e, seen)...)
			} else {
				out = append(out, types.NewTerm(false, e))
			}
			continue
		}
		for term := range union.Terms() {
			if _, isInterface := term.Type().Underlying().(*types.Interface); isInterface {
				out = append(out, writtenTerms(term.Type(), seen)...)
			} else {
				out = append(out, term)
			}
		}
	}
	return out
}
