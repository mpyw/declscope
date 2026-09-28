package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// TestTargetsInSourceOrder pins that the targets come out of the collection in
// source order, whatever order the driver hands the files in. Every stage reads
// them in that order, the baseline regeneration included, so a report and a
// baseline name the same declaration.
func TestTargetsInSourceOrder(t *testing.T) {
	fset := token.NewFileSet()
	var files []*ast.File
	// b.go is parsed and listed first, as a driver parsing concurrently may.
	for _, src := range [][2]string{
		{"b.go", "package p\n\nfunc bOne() {}\n\nfunc bTwo() {}\n"},
		{"a.go", "package p\n\nfunc aOne() {}\n"},
	} {
		f, err := parser.ParseFile(fset, src[0], src[1], parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	pkg, err := (&types.Config{}).Check("p", fset, files, info)
	if err != nil {
		t.Fatal(err)
	}
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     files,
		Pkg:       pkg,
		TypesInfo: info,
		ResultOf:  map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(files)},
	}

	opts := DefaultOptions()
	c := collectFiles(pass, opts)
	c.collectTargets(pass, opts)
	var got []string
	for _, tg := range c.targets {
		got = append(got, tg.obj.Name())
	}
	if want := []string{"aOne", "bOne", "bTwo"}; !slices.Equal(got, want) {
		t.Errorf("targets = %v, want %v", got, want)
	}
}
