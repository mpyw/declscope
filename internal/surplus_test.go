package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// TestSurplusCountsAGuessAsReach pins that a field an unkeyed literal may
// write through a guessed type set keeps its directive. go/types rejects every
// constraint whose type set cannot be computed, so no fixture that type-checks
// can make that guess. The test records the guess by hand instead, as
// collectUnkeyedFields does, and checks that surplus then stays quiet.
// implicitfield's TestGuessed pins that such a set is read as a guess.
func TestSurplusCountsAGuessAsReach(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	paths := []string{
		write("user.go", "package p\n\ntype userRow struct {\n\t//declscope:shared\n\tcount int\n}\n\nvar _ = userRow{count: 1}\n"),
		write("order.go", "package p\n\nfunc orderMake() {}\n"),
	}

	t.Run("no guess", func(t *testing.T) {
		if got := surplusFindingsOf(t, paths, false); got != 1 {
			t.Errorf("findings = %d, want 1: nothing reaches userRow.count", got)
		}
	})
	t.Run("a guess", func(t *testing.T) {
		if got := surplusFindingsOf(t, paths, true); got != 0 {
			t.Errorf("findings = %d, want 0: a guessed literal may write userRow.count", got)
		}
	})
}

// surplusFindingsOf type-checks the files at paths as package p, records a
// guessed write of the field count when guessed is set, and returns how many
// directives surplus reports.
func surplusFindingsOf(t *testing.T, paths []string, guessed bool) int {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Instances:  map[*ast.Ident]types.Instance{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
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
	c.collectRefs(pass)
	if guessed {
		var field types.Object
		for obj := range c.byObj {
			if obj.Name() == "count" {
				field = obj
			}
		}
		if field == nil {
			t.Fatal("the field count is not a target")
		}
		c.unkeyedGuessed[field] = true
	}
	s := c.computeSurplus(pass)
	if s.blind {
		t.Fatal("the pass is blind, so surplus judges nothing")
	}
	return len(s.findings)
}
