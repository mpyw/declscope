// bind.go answers one question: which comment groups a declaration takes its
// directives from. The analyzer and declscope shrink both ask it, and must
// agree, or one //declscope:ignore would bind to a declaration in one tool
// and to nothing in the other. It is go/ast positioning and holds no state
// beyond an index of the file's comments.
//
//declscope:namespace directive

package directive

import (
	"go/ast"
	"go/token"
	"slices"
)

// Binder finds the comment groups of the declarations of one file.
type Binder struct {
	fset *token.FileSet
	// lines holds the first comment group starting on each line.
	lines map[int]*ast.CommentGroup
}

// NewBinder indexes the comments of file. Lines are counted in the file as
// written, not as a //line directive renames them.
func NewBinder(fset *token.FileSet, file *ast.File) *Binder {
	b := &Binder{fset: fset, lines: map[int]*ast.CommentGroup{}}
	for _, g := range file.Comments {
		line := fset.PositionFor(g.Pos(), false).Line
		if _, seen := b.lines[line]; !seen {
			b.lines[line] = g
		}
	}
	return b
}

// Func returns the groups a func takes its directives from: its doc comment,
// and a comment trailing its first or last line. An ast.FuncDecl has no
// Comment field, so go/parser attaches neither of the latter to anything.
func (b *Binder) Func(d *ast.FuncDecl) []*ast.CommentGroup {
	return append([]*ast.CommentGroup{d.Doc}, b.loose(d)...)
}

// Block returns the groups trailing the lines of a parenthesized block's `(`
// and `)`, which reach every spec in it. The block's doc comment is not among
// them: the caller parses it on its own. Without parentheses the lines are
// the one spec's, and Spec returns them.
func (b *Binder) Block(d *ast.GenDecl) []*ast.CommentGroup {
	if !d.Lparen.IsValid() || len(d.Specs) == 0 {
		return nil
	}
	attached := slices.Concat(attachedTo(d.Specs[0]), attachedTo(d.Specs[len(d.Specs)-1]))
	return b.loose(d, attached...)
}

// Spec returns the groups a spec takes its directives from: its doc comment,
// the trailing comment go/parser attached to it, and a comment trailing its
// first or last line that the parser attached to nothing, such as one after
// the opening brace of a struct type. A comment hung on something inside the
// spec, a field's doc or trailing comment, is that field's, even when it
// shares the spec's first line.
func (b *Binder) Spec(spec ast.Spec) []*ast.CommentGroup {
	var own []*ast.CommentGroup
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.ValueSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	}
	return append(own, b.loose(spec, attachedTo(spec)...)...)
}

// loose returns the groups starting on node's first or last line, after the
// position they trail, that are not among attached.
func (b *Binder) loose(node ast.Node, attached ...*ast.CommentGroup) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, pos := range []token.Pos{node.Pos(), node.End()} {
		g, ok := b.lines[b.fset.PositionFor(pos, false).Line]
		if !ok || g.Pos() < pos || slices.Contains(attached, g) || slices.Contains(out, g) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// attachedTo returns the groups go/parser hung on a spec or on anything
// inside it, which a loose comment of the enclosing declaration never claims.
func attachedTo(spec ast.Spec) []*ast.CommentGroup {
	switch spec := spec.(type) {
	case *ast.ValueSpec:
		return []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.TypeSpec:
		out := []*ast.CommentGroup{spec.Doc, spec.Comment}
		var fields *ast.FieldList
		switch t := spec.Type.(type) {
		case *ast.StructType:
			fields = t.Fields
		case *ast.InterfaceType:
			fields = t.Methods
		}
		if fields != nil {
			for _, f := range fields.List {
				out = append(out, f.Doc, f.Comment)
			}
		}
		return out
	}
	return nil
}
