package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/mpyw/declscope/internal/rule"
)

// BenchmarkRenameGuards analyzes a package of 150 structs, each but the first
// few embedding two or three earlier ones, with ten methods apiece written in
// other files, so that every method is a qualify finding whose rename the
// guards check (#272). The package is type-checked once, outside the loop.
func BenchmarkRenameGuards(b *testing.B) {
	const structs, methods = 150, 10
	units := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet"}
	rng := rand.New(rand.NewPCG(1, 1))
	files := map[string]*strings.Builder{"types": {}, "use": {}}
	for _, unit := range units {
		files[unit] = &strings.Builder{}
	}
	for i := range structs {
		fmt.Fprintf(files["types"], "type t%d struct {\n", i)
		if i >= 3 {
			for _, j := range rng.Perm(i)[:2+rng.IntN(2)] {
				fmt.Fprintf(files["types"], "\tt%d\n", j)
			}
		}
		fmt.Fprintf(files["types"], "\tv%d int\n}\n\n", i)
		fmt.Fprintf(files["use"], "func useT%d() int {\n\tvar x t%d\n\ts := x.v%d\n", i, i, i)
		for m := range methods {
			fmt.Fprintf(files[units[(i+m)%len(units)]], "func (x *t%d) op%d() int { return x.v%d + %d }\n\n", i, m, i, m)
			fmt.Fprintf(files["use"], "\ts += x.op%d()\n", m)
		}
		fmt.Fprintf(files["use"], "\treturn s\n}\n\nvar _ = useT%d\n\n", i)
	}
	dir := b.TempDir()
	srcs := map[string]string{}
	for name, src := range files {
		srcs[filepath.Join(dir, name+".go")] = "package stress\n\n" + src.String()
	}
	pass := renameBenchPass(b, srcs)
	opts := DefaultOptions()
	opts.Qualify = rule.QualifyModeAlways
	var findings, fixes int
	for b.Loop() {
		findings, fixes = 0, 0
		pass.Report = func(d analysis.Diagnostic) {
			findings++
			fixes += len(d.SuggestedFixes)
		}
		if _, err := Run(pass, opts); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(findings), "findings")
	b.ReportMetric(float64(fixes), "fixes")
}

// TestRenameSelectsAsLookup pins that the member-name prefilter in selects
// never changes the lookup's answer: for every type the guards gather and
// every identifier of the package, selects agrees with
// types.LookupFieldOrMethod. The package holds each shape the lookup looks
// through: an embedded pointer, interface, alias and generic instance, a
// promoted method, a defined pointer type and an ambiguous selector.
func TestRenameSelectsAsLookup(t *testing.T) {
	src := `package p

type base struct{ baseField int }

func (base) baseMethod() {}

func (*base) basePtrMethod() {}

type iface interface {
	ifaceMethod()
	inner
}

type inner interface{ innerMethod() }

type gen[T any] struct {
	genField T
	*base
}

func (gen[T]) genMethod() {}

type aliased = base

type left struct{ shared int }

type right struct{ shared int }

type outer struct {
	*aliased
	iface
	gen[int]
	left
	right
	anon struct{ anonField int }
}

type ptr *outer

type viaPtr struct{ ptr int }

type deep struct{ outer }

var _ = struct {
	deep
	litField int
}{}

var _ ptr
`
	pass := renameBenchPass(t, map[string]string{filepath.Join(t.TempDir(), "p.go"): src})
	rs := &renameState{}
	rs.gatherTyped(pass)
	names := map[string]bool{"nothing": true}
	for id := range pass.TypesInfo.Defs {
		names[id.Name] = true
	}
	for _, typ := range rs.typed {
		for name := range names {
			found, index, _ := types.LookupFieldOrMethod(typ, true, pass.Pkg, name)
			if want := found != nil || index != nil; rs.selects(typ, pass.Pkg, name) != want {
				t.Errorf("selects(%v, %s) = %v, want %v", typ, name, !want, want)
			}
		}
	}
}

// renameBenchPass writes srcs, each a path and its contents, and returns a
// pass over them, type-checked.
func renameBenchPass(b testing.TB, srcs map[string]string) *analysis.Pass {
	b.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for path, src := range srcs {
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			b.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			b.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	pkg, err := (&types.Config{}).Check(files[0].Name.Name, fset, files, info)
	if err != nil {
		b.Fatal(err)
	}
	return &analysis.Pass{
		Fset:      fset,
		Files:     files,
		Pkg:       pkg,
		TypesInfo: info,
		ResultOf:  map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(files)},
	}
}
