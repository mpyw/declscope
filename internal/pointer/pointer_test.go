package pointer_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/mpyw/declscope/internal/pointer"
)

const src = `package p

type S struct{ n int }
type T struct{ n int }

type Plain *S
type Named Plain
type Alias = *S
type Value S

type Ptrs interface{ ~*S }
type Mixed interface{ ~*S | ~*T }
type Methods interface{ M() }

func f[
	One interface{ *S },
	Tilde Ptrs,
	Both interface{ Ptrs; ~*S },
	Differ Mixed,
	Open interface{ Methods },
	Wide interface{ any | *S },
	Any any,
	Val interface{ S },
]() {}
`

// TestElem pins each form a pointer takes, and the type parameters that have
// no single pointer behind them.
func TestElem(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	scope := pkg.Scope()
	typeOf := func(name string) types.Type {
		if obj := scope.Lookup(name); obj != nil {
			return obj.Type()
		}
		params := scope.Lookup("f").Type().(*types.Signature).TypeParams()
		for tp := range params.TypeParams() {
			if tp.Obj().Name() == name {
				return tp
			}
		}
		t.Fatalf("no type %s", name)
		return nil
	}
	for _, tt := range []struct {
		name string
		want string
	}{
		{"Plain", "p.S"},
		{"Named", "p.S"},
		{"Alias", "p.S"},
		{"One", "p.S"},
		{"Tilde", "p.S"},
		{"Both", "p.S"},
		{"Differ", ""},
		{"Open", ""},
		{"Wide", ""},
		{"Any", ""},
		{"Val", ""},
		{"Value", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			elem, ok := pointer.Elem(typeOf(tt.name))
			got := ""
			if ok {
				got = elem.String()
			}
			if got != tt.want {
				t.Errorf("Elem(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
