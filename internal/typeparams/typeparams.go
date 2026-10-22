// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Ported from golang.org/x/tools v0.52.0, internal/typeparams: normalize.go,
// coretype.go, and the termlist.go and typeterm.go it copies from
// cmd/compile/internal/types2. That LICENSE file reads:
//
// Copyright 2009 The Go Authors.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google LLC nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Package typeparams computes the type set of a type parameter, which
// go/types does not export. It is a port of x/tools' internal/typeparams,
// cut down to what declscope reads:
//
//   - NormalTerms is unchanged, minus its debug tracing.
//   - CoreType takes the terms NormalTerms returned, so that a caller caching
//     them per type parameter does not compute them again. It leaves out the
//     rule for directional channels: declscope asks only for a struct or a
//     pointer core type.
//   - A check no type can meet is left out, where computeTermSetInternal
//     reads an embedded type.
//   - The term and termlist operations NormalTerms does not call, and the
//     String methods only the tracing used, are left out.
package typeparams

import (
	"errors"
	"fmt"
	"go/types"
)

// errEmptyTypeSet is returned by NormalTerms for a type set holding no type.
var errEmptyTypeSet = errors.New("empty type set")

// NormalTerms returns a slice of terms representing the normalized structural
// type restrictions of a type, if any.
//
// For all types other than *types.TypeParam, *types.Interface, and
// *types.Union, this is just a single term with Tilde() == false and
// Type() == typ. For *types.TypeParam, *types.Interface, and *types.Union, see
// below.
//
// Structural type restrictions of a type parameter are created via
// non-interface types embedded in its constraint interface (directly, or via a
// chain of interface embeddings). For example, in the declaration type
// T[P interface{~int; m()}] int the structural restriction of the type
// parameter P is ~int.
//
// With interface embedding and unions, the specification of structural type
// restrictions may be arbitrarily complex. For example, consider the
// following:
//
//	type A interface{ ~string|~[]byte }
//
//	type B interface{ int|string }
//
//	type C interface { ~string|~int }
//
//	type T[P interface{ A|B; C }] int
//
// In this example, the structural type restriction of P is ~string|int: A|B
// expands to ~string|~[]byte|int|string, which reduces to ~string|~[]byte|int,
// which when intersected with C (~string|~int) yields ~string|int.
//
// NormalTerms computes these expansions and reductions, producing a
// "normalized" form of the embeddings. A structural restriction is normalized
// if it is a single union containing no interface terms, and is minimal in the
// sense that removing any term changes the set of types satisfying the
// constraint. It is left as a proof for the reader that, modulo sorting, there
// is exactly one such normalized form.
//
// Because the minimal representation always takes this form, NormalTerms
// returns a slice of tilde terms corresponding to the terms of the union in
// the normalized structural restriction. An error is returned if the type is
// invalid, exceeds complexity bounds, or has an empty type set. In the latter
// case, NormalTerms returns errEmptyTypeSet.
//
// NormalTerms makes no guarantees about the order of terms, except that it
// is deterministic.
func NormalTerms(T types.Type) ([]*types.Term, error) {
	// typeSetOf(T) == typeSetOf(Unalias(T))
	typ := types.Unalias(T)
	if named, ok := typ.(*types.Named); ok {
		typ = named.Underlying()
	}
	switch typ := typ.(type) {
	case *types.TypeParam:
		return structuralTerms(typ)
	case *types.Union:
		return computeTermSet(typ)
	case *types.Interface:
		return computeTermSet(typ)
	default:
		return []*types.Term{types.NewTerm(false, T)}, nil
	}
}

// CoreType returns the core type of the type set terms describes, as
// NormalTerms returns it, or nil if it has none: the one underlying type
// every type in the set has.
//
// As of Go1.25, the notion of a core type has been removed from the language
// spec. See https://go.dev/blog/coretypes for more details. A composite
// literal of a type parameter still needs one.
func CoreType(terms []*types.Term) types.Type {
	if len(terms) == 0 {
		// An empty list is a type set that terms do not limit.
		return nil
	}
	U := terms[0].Type().Underlying()
	for _, t := range terms[1:] {
		if !types.Identical(U, t.Type().Underlying()) {
			return nil
		}
	}
	// From the deprecated core types spec:
	// "There is a single type U which is the underlying type of all types in the type set of T"
	return U
}

// structuralTerms returns the normalized terms of a type parameter's
// constraint, as NormalTerms describes them. x/tools also checks that the
// constraint is a non-nil interface. go/types guarantees both: it panics on a
// nil constraint, and wraps a non-interface one in an implicit interface.
func structuralTerms(tparam *types.TypeParam) ([]*types.Term, error) {
	return computeTermSet(tparam.Constraint())
}

func computeTermSet(typ types.Type) ([]*types.Term, error) {
	tset, err := computeTermSetInternal(typ, make(map[types.Type]*termSet))
	if err != nil {
		return nil, err
	}
	if tset.terms.isEmpty() {
		return nil, errEmptyTypeSet
	}
	if tset.terms.isAll() {
		return nil, nil
	}
	var terms []*types.Term
	for _, term := range tset.terms {
		terms = append(terms, types.NewTerm(term.tilde, term.typ))
	}
	return terms, nil
}

// A termSet holds the normalized set of terms for a given type.
//
// The name termSet is intentionally distinct from 'type set': a type set is
// all types that implement a type (and includes method restrictions), whereas
// a term set just represents the structural restrictions on a type.
type termSet struct {
	complete bool
	terms    termlist
}

func computeTermSetInternal(t types.Type, seen map[types.Type]*termSet) (res *termSet, err error) {
	const maxTermCount = 100
	if tset, ok := seen[t]; ok {
		if !tset.complete {
			return nil, fmt.Errorf("cycle detected in the declaration of %s", t)
		}
		return tset, nil
	}

	// Mark the current type as seen to avoid infinite recursion.
	tset := new(termSet)
	defer func() {
		tset.complete = true
	}()
	seen[t] = tset

	switch u := t.Underlying().(type) {
	case *types.Interface:
		// The term set of an interface is the intersection of the term sets of its
		// embedded types.
		tset.terms = allTermlist
		// x/tools refuses an embedded type whose underlying type is a type
		// parameter here. None has one: the underlying type of a type
		// parameter is its constraint interface. So the check is left out.
		for embedded := range u.EmbeddedTypes() {
			tset2, err := computeTermSetInternal(embedded, seen)
			if err != nil {
				return nil, err
			}
			tset.terms = tset.terms.intersect(tset2.terms)
		}
	case *types.Union:
		// The term set of a union is the union of term sets of its terms.
		tset.terms = nil
		for t := range u.Terms() {
			var terms termlist
			switch t.Type().Underlying().(type) {
			case *types.Interface:
				tset2, err := computeTermSetInternal(t.Type(), seen)
				if err != nil {
					return nil, err
				}
				terms = tset2.terms
			case *types.TypeParam, *types.Union:
				// A stand-alone type parameter or union is not permitted as union
				// term.
				return nil, fmt.Errorf("invalid union term %T", t)
			default:
				if t.Type() == types.Typ[types.Invalid] {
					continue
				}
				terms = termlist{{t.Tilde(), t.Type()}}
			}
			tset.terms = tset.terms.union(terms)
			if len(tset.terms) > maxTermCount {
				return nil, fmt.Errorf("exceeded max term count %d", maxTermCount)
			}
		}
	default:
		// For all other types, the term set is just a single non-tilde term
		// holding the type itself.
		if u != types.Typ[types.Invalid] {
			tset.terms = termlist{{false, t}}
		}
	}
	return tset, nil
}

// A termlist represents the type set represented by the union
// t1 ∪ y2 ∪ ... tn of the type sets of the terms t1 to tn.
// A termlist is in normal form if all terms are disjoint.
// termlist operations don't require the operands to be in
// normal form.
type termlist []*term

// allTermlist represents the set of all types.
// It is in normal form.
var allTermlist = termlist{new(term)}

// isEmpty reports whether the termlist xl represents the empty set of types.
func (xl termlist) isEmpty() bool {
	// If there's a non-nil term, the entire list is not empty.
	// If the termlist is in normal form, this requires at most
	// one iteration.
	for _, x := range xl {
		if x != nil {
			return false
		}
	}
	return true
}

// isAll reports whether the termlist xl represents the set of all types.
func (xl termlist) isAll() bool {
	// If there's a 𝓤 term, the entire list is 𝓤.
	// If the termlist is in normal form, this requires at most
	// one iteration.
	for _, x := range xl {
		if x != nil && x.typ == nil {
			return true
		}
	}
	return false
}

// norm returns the normal form of xl.
func (xl termlist) norm() termlist {
	// Quadratic algorithm, but good enough for now.
	// TODO(gri) fix asymptotic performance
	used := make([]bool, len(xl))
	var rl termlist
	for i, xi := range xl {
		if xi == nil || used[i] {
			continue
		}
		for j := i + 1; j < len(xl); j++ {
			xj := xl[j]
			if xj == nil || used[j] {
				continue
			}
			if u1, u2 := xi.union(xj); u2 == nil {
				// If we encounter a 𝓤 term, the entire list is 𝓤.
				// Exit early.
				// (Note that this is not just an optimization;
				// if we continue, we may end up with a 𝓤 term
				// and other terms and the result would not be
				// in normal form.)
				if u1.typ == nil {
					return allTermlist
				}
				xi = u1
				used[j] = true // xj is now unioned into xi - ignore it in future iterations
			}
		}
		rl = append(rl, xi)
	}
	return rl
}

// union returns the union xl ∪ yl.
func (xl termlist) union(yl termlist) termlist {
	return append(xl, yl...).norm()
}

// intersect returns the intersection xl ∩ yl.
func (xl termlist) intersect(yl termlist) termlist {
	if xl.isEmpty() || yl.isEmpty() {
		return nil
	}

	// Quadratic algorithm, but good enough for now.
	// TODO(gri) fix asymptotic performance
	var rl termlist
	for _, x := range xl {
		for _, y := range yl {
			if r := x.intersect(y); r != nil {
				rl = append(rl, r)
			}
		}
	}
	return rl.norm()
}

// A term describes elementary type sets:
//
//	 ∅:  (*term)(nil)     == ∅                      // set of no types (empty set)
//	 𝓤:  &term{}          == 𝓤                      // set of all types (𝓤niverse)
//	 T:  &term{false, T}  == {T}                    // set of type T
//	~t:  &term{true, t}   == {t' | under(t') == t}  // set of types with underlying type t
type term struct {
	tilde bool // valid if typ != nil
	typ   types.Type
}

// union returns the union x ∪ y: zero, one, or two non-nil terms.
func (x *term) union(y *term) (_, _ *term) {
	// easy cases
	switch {
	case x == nil && y == nil:
		return nil, nil // ∅ ∪ ∅ == ∅
	case x == nil:
		return y, nil // ∅ ∪ y == y
	case y == nil:
		return x, nil // x ∪ ∅ == x
	case x.typ == nil:
		return x, nil // 𝓤 ∪ y == 𝓤
	case y.typ == nil:
		return y, nil // x ∪ 𝓤 == 𝓤
	}
	// ∅ ⊂ x, y ⊂ 𝓤

	if x.disjoint(y) {
		return x, y // x ∪ y == (x, y) if x ∩ y == ∅
	}
	// x.typ == y.typ

	// ~t ∪ ~t == ~t
	// ~t ∪  T == ~t
	//  T ∪ ~t == ~t
	//  T ∪  T ==  T
	if x.tilde || !y.tilde {
		return x, nil
	}
	return y, nil
}

// intersect returns the intersection x ∩ y.
func (x *term) intersect(y *term) *term {
	// easy cases
	switch {
	case x == nil || y == nil:
		return nil // ∅ ∩ y == ∅ and ∩ ∅ == ∅
	case x.typ == nil:
		return y // 𝓤 ∩ y == y
	case y.typ == nil:
		return x // x ∩ 𝓤 == x
	}
	// ∅ ⊂ x, y ⊂ 𝓤

	if x.disjoint(y) {
		return nil // x ∩ y == ∅ if x ∩ y == ∅
	}
	// x.typ == y.typ

	// ~t ∩ ~t == ~t
	// ~t ∩  T ==  T
	//  T ∩ ~t ==  T
	//  T ∩  T ==  T
	if !x.tilde || y.tilde {
		return x
	}
	return y
}

// disjoint reports whether x ∩ y == ∅.
// x.typ and y.typ must not be nil.
func (x *term) disjoint(y *term) bool {
	ux := x.typ
	if y.tilde {
		ux = ux.Underlying()
	}
	uy := y.typ
	if x.tilde {
		uy = uy.Underlying()
	}
	return !types.Identical(ux, uy)
}
