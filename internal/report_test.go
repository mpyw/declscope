package internal

import (
	"errors"
	"go/token"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// TestAtLineStartFailsSafe pins that a position the pass cannot read around
// is not called the start of a line, so a directive fix breaks the line
// rather than being written after code.
func TestAtLineStartFailsSafe(t *testing.T) {
	const content = "package x\n\ntype t struct{ n int }\n"
	fset := token.NewFileSet()
	tf := fset.AddFile("a.go", -1, len(content))
	tf.SetLinesForContent([]byte(content))
	pass := &analysis.Pass{
		Fset:     fset,
		ReadFile: func(string) ([]byte, error) { return []byte(content), nil },
		ResultOf: map[*analysis.Analyzer]any{inspect.Analyzer: inspector.New(nil)},
	}
	c := &collection{}
	if !c.atLineStartForReport(pass, tf.Pos(strings.Index(content, "type"))) {
		t.Fatal("a declaration at the start of its line was not recognised")
	}
	field := tf.Pos(strings.Index(content, "n int"))
	if c.atLineStartForReport(pass, field) {
		t.Fatal("a field after code was called the start of its line")
	}
	for name, read := range map[string]func(string) ([]byte, error){
		"a read error":                      func(string) ([]byte, error) { return nil, errors.New("gone") },
		"a file shorter than it was parsed": func(string) ([]byte, error) { return []byte("package x\n"), nil },
	} {
		// A fresh collection for each, since a pass reads a file only once.
		pass.ReadFile = read
		if (&collection{}).atLineStartForReport(pass, field) {
			t.Errorf("%s: the position was called the start of its line", name)
		}
	}
}
