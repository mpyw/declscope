package pointer_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal/pointer"
)

// The oracle declares structs in package a and writes them from package b
// through a type parameter. Unexporting the fields of one struct breaks b
// exactly when b writes them without naming them, which is what declscope
// shrink would do and what the analyzer must see as a use. go/types decides
// both: which programs are valid, and which of them a rename breaks.

// oracleStructs are the structs whose fields the oracle unexports, one at a
// time.
var oracleStructs = []string{"S", "T", "R", "G"}

// oracleSource returns package a, with the fields of the structs hidden names
// unexported, as S or S+T.
func oracleSource(hidden string) string {
	field := func(name string) (string, string) {
		if slices.Contains(strings.Split(hidden, "+"), name) {
			return "x", "y"
		}
		return "X", "Y"
	}
	var b strings.Builder
	b.WriteString("package a\n\n")
	sx, sy := field("S")
	fmt.Fprintf(&b, "type S struct{ %s, %s int }\n", sx, sy)
	tx, ty := field("T")
	fmt.Fprintf(&b, "type T struct{ %s, %s int }\n", tx, ty)
	rx, ry := field("R")
	fmt.Fprintf(&b, "type R struct {\n\t%s int `json:\"x\"`\n\t%s int\n}\n", rx, ry)
	gx, gy := field("G")
	fmt.Fprintf(&b, "type G[E any] struct{ %s, %s E }\n", gx, gy)
	b.WriteString(`
type DS S
type PS *S
type PT *T
type PPS PS
type AS = S
type APS = *S
type IS interface{ S }
type IPS interface{ ~*S }
type IST interface{ S | T }
type AIS = IS
type PtrTo[E any] interface{ *E }
type Either[E, F any] interface{ *E | *F }
type Under[E any] interface{ ~*E; *S | *T }
type Nested interface{ interface{ IPS | *T }; ~*S }
`)
	return b.String()
}

// oracleTerms are the terms the constraints are built from: every form a
// struct and a pointer to one take in a constraint.
var oracleTerms = []string{
	"a.S", "a.T", "a.R", "a.G[int]", "a.DS", "a.AS", "~struct{ X, Y int }",
	"*a.S", "*a.T", "~*a.S", "*a.G[int]", "a.PS", "a.PT", "a.PPS", "a.APS",
	"a.IS", "a.IPS", "a.IST", "a.AIS", "a.PtrTo[a.S]", "a.PtrTo[a.G[int]]",
	"a.Either[a.S, a.T]", "a.Under[a.S]", "a.Nested",
}

// oracleConstraints builds the constraints: each term alone, every union of
// two, every intersection of two, a union intersected with one of its own
// terms, and both nested inside interfaces.
func oracleConstraints() []string {
	var out []string
	for _, x := range oracleTerms {
		out = append(out, "interface{ "+x+" }", "interface{ interface{ "+x+" } }")
	}
	for i, x := range oracleTerms {
		for _, y := range oracleTerms[i+1:] {
			out = append(out,
				"interface{ "+x+" | "+y+" }",
				"interface{ "+x+"; "+y+" }",
				"interface{ "+x+" | "+y+"; "+x+" }",
				"interface{ "+y+" | "+x+"; "+y+" }",
				"interface{ interface{ "+x+" | "+y+" }; interface{ "+y+" } }",
				"interface{ interface{ "+x+" } | "+y+"; comparable }",
			)
		}
	}
	return out
}

// oracleForms are the expressions written through the type parameter P, from
// u of type U and p of type P.
var oracleForms = []struct {
	name, expr string
	literal    bool
}{
	{"literal", "P{1, 2}", true},
	{"elided", "[]P{{1, 2}}", true},
	{"elided pointer", "[]*P{{1, 2}}", true},
	{"conversion", "P(u)", false},
	{"pointer conversion", "P(&u)", false},
	{"conversion from P", "U(p)", false},
	{"pointer conversion from P", "(*U)(p)", false},
}

// oraclePairs returns every two of oracleStructs, as S+T: shrink unexports
// the fields of several structs in one run.
func oraclePairs() []string {
	var out []string
	for i, x := range oracleStructs {
		for _, y := range oracleStructs[i+1:] {
			out = append(out, x+"+"+y)
		}
	}
	return out
}

type oracle struct {
	fset     *token.FileSet
	variants map[string]*types.Package
}

func newOracle(t *testing.T) *oracle {
	o := &oracle{fset: token.NewFileSet(), variants: map[string]*types.Package{}}
	for _, hidden := range append([]string{""}, slices.Concat(oracleStructs, oraclePairs())...) {
		f, err := parser.ParseFile(o.fset, "a.go", oracleSource(hidden), 0)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := (&types.Config{}).Check("example.com/a", o.fset, []*ast.File{f}, nil)
		if err != nil {
			t.Fatalf("package a with %s hidden: %v", hidden, err)
		}
		o.variants[hidden] = pkg
	}
	return o
}

// check type-checks package b against package a with hidden's fields
// unexported, and returns the expression written through P.
func (o *oracle) check(constraint, expr, hidden string) (ast.Expr, *types.Info, error) {
	src := fmt.Sprintf("package b\n\nimport \"example.com/a\"\n\ntype U struct{ X, Y int }\n\nfunc f[P %s](u U, p P) {\n\t_ = %s\n}\n", constraint, expr)
	f, err := parser.ParseFile(o.fset, "b.go", src, 0)
	if err != nil {
		return nil, nil, err
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := &types.Config{Importer: importerFunc(func(string) (*types.Package, error) { return o.variants[hidden], nil })}
	if _, err := conf.Check("example.com/b", o.fset, []*ast.File{f}, info); err != nil {
		return nil, nil, err
	}
	fn := f.Decls[len(f.Decls)-1].(*ast.FuncDecl)
	return fn.Body.List[0].(*ast.AssignStmt).Rhs[0], info, nil
}

type importerFunc func(string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// read returns the structs of package a whose fields declscope reads the
// expression as writing, by name, and whether that is exact.
func (o *oracle) read(sets *pointer.Sets, expr ast.Expr, info *types.Info, literal bool) (map[string]bool, bool) {
	var holders []types.Type
	exact := true
	if literal {
		lit := expr.(*ast.CompositeLit)
		if inner, ok := lit.Elts[0].(*ast.CompositeLit); ok {
			lit = inner
		}
		holders, exact = sets.Literal(info.TypeOf(lit))
	} else {
		call := expr.(*ast.CallExpr)
		for _, side := range []ast.Expr{call.Fun, call.Args[0]} {
			h, ok := sets.Conversion(info.TypeOf(side))
			holders, exact = append(holders, h...), exact && ok
		}
	}
	scope := o.variants[""].Scope()
	got := map[string]bool{}
	for _, h := range holders {
		field := h.Underlying().(*types.Struct).Field(0).Origin()
		for _, name := range oracleStructs {
			if scope.Lookup(name).Type().Underlying().(*types.Struct).Field(0) == field {
				got[name] = true
			}
		}
	}
	return got, exact
}

// TestOracle asserts that for every constraint and every expression go/types
// accepts, the structs declscope reads it as writing are exactly the ones
// whose unexported fields break it.
func TestOracle(t *testing.T) {
	o := newOracle(t)
	var sets pointer.Sets
	accepted, writing, wider := 0, 0, 0
	for _, constraint := range oracleConstraints() {
		for _, form := range oracleForms {
			expr, info, err := o.check(constraint, form.expr, "")
			if err != nil {
				continue
			}
			accepted++
			want := map[string]bool{}
			for _, name := range oracleStructs {
				if _, _, err := o.check(constraint, form.expr, name); err != nil {
					want[name] = true
				}
			}
			if len(want) > 0 {
				writing++
			}
			got, exact := o.read(&sets, expr, info, form.literal)
			if !exact {
				t.Errorf("%s through %s: the type set was guessed", form.expr, constraint)
			}
			if slices.ContainsFunc(keys(want), func(name string) bool { return !got[name] }) {
				t.Errorf("%s through %s: read as writing %v, but unexporting %v breaks it", form.expr, constraint, keys(got), keys(want))
			}
			if !mapsEqual(got, want) {
				wider++
			}
			// Two fixed at once may break what neither breaks alone. One of
			// them must then be read.
			for _, pair := range oraclePairs() {
				x, y, _ := strings.Cut(pair, "+")
				if got[x] || got[y] {
					continue
				}
				if _, _, err := o.check(constraint, form.expr, pair); err != nil {
					t.Errorf("%s through %s: read as writing %v, but unexporting %s breaks it", form.expr, constraint, keys(got), pair)
				}
			}
		}
	}
	t.Logf("%d expressions accepted, %d of them write a struct of package a, %d read wider", accepted, writing, wider)
	// The audit behind the issue tried 16 shapes. Far more pass here, and
	// most of them write one.
	if writing < 200 {
		t.Errorf("only %d accepted expressions write a struct of package a", writing)
	}
}

func mapsEqual(a, b map[string]bool) bool {
	return len(a) == len(b) && !slices.ContainsFunc(keys(a), func(k string) bool { return !b[k] })
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestGuessed pins a type set that cannot be computed: a union over the term
// limit, which only a constraint go/types never checked can hold. Every
// struct the constraint writes is returned, behind its pointer too, and the
// result is not exact.
func TestGuessed(t *testing.T) {
	pkg := types.NewPackage("example.com/p", "p")
	var terms []*types.Term
	var structs []types.Type
	for i := range 101 {
		st := types.NewStruct([]*types.Var{types.NewField(token.NoPos, pkg, "X", types.Typ[types.Int], false)}, nil)
		named := types.NewNamed(types.NewTypeName(token.NoPos, pkg, fmt.Sprintf("S%d", i), nil), st, nil)
		structs = append(structs, named)
		terms = append(terms, types.NewTerm(false, types.NewPointer(named)))
	}
	inner := types.NewInterfaceType(nil, []types.Type{types.NewUnion(terms)})
	constraint := types.NewInterfaceType(nil, []types.Type{inner, types.NewUnion([]*types.Term{types.NewTerm(false, inner)})})
	tp := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "P", nil), constraint)
	var sets pointer.Sets
	for _, ask := range []func(types.Type) ([]types.Type, bool){sets.Literal, sets.Conversion} {
		got, exact := ask(tp)
		if exact {
			t.Error("a type set over the term limit was read as exact")
		}
		if !slices.Equal(got, structs) {
			t.Errorf("got %d structs, want the %d the constraint writes", len(got), len(structs))
		}
	}
	// Through a pointer too: the literal of []*P{{1}} leaves out *P.
	if got, exact := sets.Literal(types.NewPointer(tp)); exact || len(got) != len(structs) {
		t.Errorf("Literal(*P) = %d structs, exact %v", len(got), exact)
	}
}

// TestNotStructs pins the types that write no struct's fields.
func TestNotStructs(t *testing.T) {
	var sets pointer.Sets
	for _, typ := range []types.Type{
		types.Typ[types.Int],
		types.NewPointer(types.Typ[types.Int]),
		types.NewSlice(types.Typ[types.Int]),
	} {
		if got, exact := sets.Literal(typ); len(got) != 0 || !exact {
			t.Errorf("Literal(%s) = %v, %v", typ, got, exact)
		}
		if got, exact := sets.Conversion(typ); len(got) != 0 || !exact {
			t.Errorf("Conversion(%s) = %v, %v", typ, got, exact)
		}
	}
}

// TestLeftOut pins what a literal reads behind the one pointer it may leave
// out: a struct, and not a second pointer, which a type parameter of pointers
// would be behind *P.
func TestLeftOut(t *testing.T) {
	pkg := types.NewPackage("example.com/p", "p")
	st := types.NewStruct([]*types.Var{types.NewField(token.NoPos, pkg, "X", types.Typ[types.Int], false)}, nil)
	s := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "S", nil), st, nil)
	ptrs := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "P", nil), types.NewInterfaceType(nil, []types.Type{types.NewPointer(s)}))
	var sets pointer.Sets
	if got, exact := sets.Literal(s); len(got) != 1 || got[0] != s || !exact {
		t.Errorf("Literal(S) = %v, %v", got, exact)
	}
	if got, exact := sets.Literal(types.NewPointer(ptrs)); len(got) != 0 || !exact {
		t.Errorf("Literal(*P) = %v, %v, with P allowing only *S", got, exact)
	}
}

// BenchmarkSharedUnion reads the literals of 600 type parameters, three in
// each of 200 functions, all constrained by one union of 99 structs (#339).
// The set is computed once for the constraint, not once per type parameter,
// and the union grows by inserting each term. The package is type-checked
// once, outside the loop.
func BenchmarkSharedUnion(b *testing.B) {
	const structs, funcs, params = 99, 200, 3
	var src strings.Builder
	src.WriteString("package p\n\n")
	union := make([]string, structs)
	for i := range structs {
		fmt.Fprintf(&src, "type S%d struct{ A, B int }\n", i)
		union[i] = fmt.Sprintf("S%d", i)
	}
	fmt.Fprintf(&src, "\ntype C interface{ %s }\n\n", strings.Join(union, " | "))
	for i := range funcs {
		fmt.Fprintf(&src, "func G%d[P0, P1, P2 C]() {}\n", i)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src.String(), 0)
	if err != nil {
		b.Fatal(err)
	}
	pkg, err := (&types.Config{}).Check("example.com/p", fset, []*ast.File{f}, nil)
	if err != nil {
		b.Fatal(err)
	}
	var tparams []types.Type
	for i := range funcs {
		sig := pkg.Scope().Lookup(fmt.Sprintf("G%d", i)).Type().(*types.Signature)
		for tp := range sig.TypeParams().TypeParams() {
			tparams = append(tparams, tp)
		}
	}
	if len(tparams) != funcs*params {
		b.Fatalf("got %d type parameters", len(tparams))
	}
	for b.Loop() {
		var sets pointer.Sets
		for _, tp := range tparams {
			if got, exact := sets.Literal(tp); len(got) != structs || !exact {
				b.Fatalf("Literal(%s) = %d structs, exact %v", tp, len(got), exact)
			}
		}
	}
}
