// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Ported from golang.org/x/tools v0.52.0, internal/typeparams:
// normalize_test.go and coretype_test.go. The license is the one quoted in
// typeparams.go.

package typeparams

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"testing"
)

func TestStructuralTerms(t *testing.T) {
	// In the following tests, src must define a type T with (at least) one type
	// parameter. We will compute the structural terms of the first type
	// parameter.
	tests := []struct {
		src       string
		want      string
		wantError string
	}{
		{"package emptyinterface0; type T[P interface{}] int", "all", ""},
		{"package emptyinterface1; type T[P interface{ int | interface{} }] int", "all", ""},
		{"package singleton; type T[P interface{ int }] int", "int", ""},
		{"package under; type T[P interface{~int}] int", "~int", ""},
		{"package superset; type T[P interface{ ~int | int }] int", "~int", ""},
		{"package overlap; type T[P interface{ ~int; int }] int", "int", ""},
		{"package emptyintersection; type T[P interface{ ~int; string }] int", "", "empty type set"},

		{"package embedded0; type T[P interface{ I }] int; type I interface { int }", "int", ""},
		{"package embedded1; type T[P interface{ I | string }] int; type I interface{ int | ~string }", "int ?\\| ?~string", ""},
		{"package embedded2; type T[P interface{ I; string }] int; type I interface{ int | ~string }", "string", ""},

		{"package named; type T[P C] int; type C interface{ ~int|int }", "~int", ""},
		{`// package example is taken from the docstring for StructuralTerms
package example

type A interface{ ~string|~[]byte }

type B interface{ int|string }

type C interface { ~string|~int }

type T[P interface{ A|B; C }] int
`, "~string ?\\| ?int", ""},
		// Added for declscope: a term go/types could not resolve, and an
		// interface reached twice.
		{"package invalid; type T[P interface{ Missing | int }] int", "int", ""},
		{"package twice; type T[P interface{ I | J }] int; type I interface{ ~int }; type J interface{ I | string }", "~int ?\\| ?string", ""},
	}

	for _, test := range tests {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "p.go", test.src, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(f.Name.Name, func(t *testing.T) {
			conf := types.Config{
				Error: func(error) {}, // keep going on errors
			}
			pkg, err := conf.Check("", fset, []*ast.File{f}, nil)
			if err != nil {
				t.Logf("types.Config.Check: %v", err)
				// keep going on type checker errors: we want to assert on behavior of
				// invalid code as well.
			}
			obj := pkg.Scope().Lookup("T")
			if obj == nil {
				t.Fatal("type T not found")
			}
			T := obj.Type().(*types.Named).TypeParams().At(0)
			terms, err := NormalTerms(T)
			if test.wantError != "" {
				if err == nil {
					t.Fatalf("NormalTerms(%s): nil error, want %q", T, test.wantError)
				}
				if !strings.Contains(err.Error(), test.wantError) {
					t.Errorf("NormalTerms(%s): err = %q, want %q", T, err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got string
			if len(terms) == 0 {
				got = "all"
			} else {
				qual := types.RelativeTo(pkg)
				got = types.TypeString(types.NewUnion(terms), qual)
			}
			want := regexp.MustCompile(test.want)
			if !want.MatchString(got) {
				t.Errorf("NormalTerms(%s) = %q, want %q", T, got, test.want)
			}
		})
	}
}

// TestInvalidConstraints pins the constraints NormalTerms refuses. go/types
// replaces each with a valid one when it reports the error, so they are
// built directly. A type parameter embedded in its constraint or written as a
// union term cannot be built even so: go/types asserts against both.
func TestInvalidConstraints(t *testing.T) {
	// NewTypeParam computes the type set of its constraint, which an invalid
	// one fails; SetConstraint does not.
	param := func(constraint types.Type) *types.TypeParam {
		tp := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "P", nil), types.NewInterfaceType(nil, nil))
		tp.SetConstraint(constraint)
		return tp
	}
	// I embeds J, whose underlying interface embeds I again.
	j := types.NewNamed(types.NewTypeName(token.NoPos, nil, "J", nil), nil, nil)
	i := types.NewInterfaceType(nil, []types.Type{j})
	j.SetUnderlying(types.NewInterfaceType(nil, []types.Type{i}))
	var many []*types.Term
	for n := range 101 {
		many = append(many, types.NewTerm(false, types.NewArray(types.Typ[types.Int], int64(n))))
	}
	for _, test := range []struct {
		name       string
		constraint types.Type
		wantError  string
	}{
		{"cycle", i, "cycle detected"},
		{"too many terms", types.NewInterfaceType(nil, []types.Type{types.NewUnion(many)}), "exceeded max term count"},
		{"too many in an interface term", types.NewInterfaceType(nil, []types.Type{types.NewUnion([]*types.Term{
			types.NewTerm(false, types.NewInterfaceType(nil, []types.Type{types.NewUnion(many)})),
		})}), "exceeded max term count"},
		{"only invalid", types.NewInterfaceType(nil, []types.Type{types.Typ[types.Invalid]}), "empty type set"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NormalTerms(param(test.constraint)); err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("NormalTerms = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestCoreType(t *testing.T) {
	const source = `
	package P

	type Named int

	type A any
	type B interface{~int}
	type C interface{int}
	type D interface{Named}
	type E interface{~int|interface{Named}}
	type F interface{~int|~float32}
	type G interface{chan int|interface{chan int}}
	type H interface{chan int|chan float32}
	type M interface{chan int|chan Named}
	type O interface{chan int|bool}
	type P struct{ Named }
	type Q interface{ Foo() }
	type R interface{ Foo() ; Named }
	type S interface{ Foo() ; ~int }
	type U = interface{ *P | ~*struct{ Named } }
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "hello.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	var conf types.Config
	pkg, err := conf.Check("P", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// The directional channel cases are left out with the rule for them.
	for _, test := range []struct {
		expr string // type expression of Named type
		want string // expected core type (or "<nil>" if none)
	}{
		{"Named", "int"},         // Underlying type is not interface.
		{"A", "<nil>"},           // Interface has no terms.
		{"B", "int"},             // Tilde term.
		{"C", "int"},             // Non-tilde term.
		{"D", "int"},             // Named term.
		{"E", "int"},             // Identical underlying types.
		{"F", "<nil>"},           // Differing underlying types.
		{"G", "chan int"},        // Identical Element types.
		{"H", "<nil>"},           // Element type int has differing underlying type to float32.
		{"M", "<nil>"},           // Element type int is not *identical* to Named.
		{"O", "<nil>"},           // A channel followed by a non-channel.
		{"P", "struct{P.Named}"}, // Embedded type.
		{"Q", "<nil>"},           // interface type with no terms and functions
		{"R", "int"},             // interface type with both terms and functions.
		{"S", "int"},             // interface type with a tilde term
		{"U", "<nil>"},           // Added for declscope: an alias, and pointers to two types.
	} {
		// Eval() expr for its type.
		tv, err := types.Eval(fset, pkg, 0, test.expr)
		if err != nil {
			t.Fatalf("Eval(%s) failed: %v", test.expr, err)
		}

		terms, err := NormalTerms(tv.Type)
		if err != nil {
			t.Fatalf("NormalTerms(%s) failed: %v", test.expr, err)
		}
		ct := CoreType(terms)
		var got string
		if ct == nil {
			got = "<nil>"
		} else {
			got = ct.String()
		}
		if got != test.want {
			t.Errorf("CoreType(%s) = %v, want %v", test.expr, got, test.want)
		}
	}
}

// TestUnionTerms pins a union passed on its own, which NormalTerms reads as
// the interface embedding it would.
func TestUnionTerms(t *testing.T) {
	union := types.NewUnion([]*types.Term{types.NewTerm(true, types.Typ[types.Int]), types.NewTerm(false, types.Typ[types.Int])})
	terms, err := NormalTerms(union)
	if err != nil || len(terms) != 1 || !terms[0].Tilde() {
		t.Errorf("NormalTerms(~int | int) = %v, %v, want ~int", terms, err)
	}
}

// TestTerms pins the term operations on every pair of the sets they handle:
// none, all, one type, and every type with one underlying type. Ported from
// the table cmd/compile/internal/types2 tests them with.
func TestTerms(t *testing.T) {
	myInt := types.NewNamed(types.NewTypeName(token.NoPos, nil, "myInt", nil), types.Typ[types.Int], nil)
	terms := map[string]*term{
		"∅":      nil,
		"𝓤":      {},
		"int":    {false, types.Typ[types.Int]},
		"~int":   {true, types.Typ[types.Int]},
		"string": {false, types.Typ[types.String]},
		"myInt":  {false, myInt},
	}
	for _, test := range []struct {
		x, y, union, intersect string
	}{
		{"∅", "∅", "∅ ∅", "∅"},
		{"∅", "int", "int ∅", "∅"},
		{"int", "∅", "int ∅", "∅"},
		{"𝓤", "int", "𝓤 ∅", "int"},
		{"int", "𝓤", "𝓤 ∅", "int"},
		{"int", "string", "int string", "∅"},
		{"int", "~int", "~int ∅", "int"},
		{"~int", "int", "~int ∅", "int"},
		{"~int", "myInt", "~int ∅", "myInt"},
		{"myInt", "~int", "~int ∅", "myInt"},
		{"int", "int", "int ∅", "int"},
	} {
		x, y := terms[test.x], terms[test.y]
		u1, u2 := x.union(y)
		if got := termName(terms, u1) + " " + termName(terms, u2); got != test.union {
			t.Errorf("%s ∪ %s = %s, want %s", test.x, test.y, got, test.union)
		}
		if got := termName(terms, x.intersect(y)); got != test.intersect {
			t.Errorf("%s ∩ %s = %s, want %s", test.x, test.y, got, test.intersect)
		}
	}
	// A list holding 𝓤 is 𝓤, and an empty one intersects to nothing.
	if l := (termlist{terms["int"], terms["𝓤"]}).norm(); !l.isAll() {
		t.Errorf("int | 𝓤 normalizes to %d terms, not 𝓤", len(l))
	}
	if l := (termlist{}).intersect(allTermlist); l != nil {
		t.Errorf("∅ ∩ 𝓤 = %d terms", len(l))
	}
}

func termName(terms map[string]*term, x *term) string {
	for name, t := range terms {
		if t == x {
			return name
		}
	}
	return "?"
}
