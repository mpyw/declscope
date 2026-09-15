// comments.go answers one question: which comment groups belong to this
// node. It is go/ast positioning and holds no state. Collect.go keeps the
// part that does hold state, namely which groups a declaration has already
// taken its directives from. That accounting belongs to the stage, not to
// the question.
//
// The namespace is a noun so that a prefix reads as one: commentsAttached
// names a thing, where a verb namespace would have made it an instruction.

package internal

import (
	"go/ast"
	"go/token"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// commentsForSpec returns the comment groups a spec takes its directives from: its
// doc comment, the trailing comment go/parser attached to it, and any comment
// trailing its first or last line that the parser attached to nothing, such
// as one after the opening brace of a struct type. A comment the parser hung
// on something inside the spec — a field's doc or trailing comment — is that
// field's and is left for addMembersToCollection, even when it shares the spec's first
// line.
//
//declscope:package // collect.go asks this while walking a GenDecl
func commentsForSpec(pass *analysis.Pass, fi *fileInfo, spec ast.Spec) []*ast.CommentGroup {
	var own []*ast.CommentGroup
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.ValueSpec:
		own = []*ast.CommentGroup{spec.Doc, spec.Comment}
	}
	return append(own, fi.looseTrailingComments(pass.Fset, spec, commentsAttached(spec)...)...)
}

// commentsTrailingAt returns the comment group trailing the declaration starting at
// pos, if any.
func (f *fileInfo) commentsTrailingAt(fset *token.FileSet, pos token.Pos) *ast.CommentGroup {
	g, ok := f.lineComments[fset.Position(pos).Line]
	if !ok || g.Pos() < pos {
		return nil
	}
	return g
}

// looseTrailingComments returns the comment groups on the first and last lines of
// node that go/parser attached to nothing — one after the opening brace of a
// struct type, after the parenthesis of a block, or after a function's
// closing brace. They belong to the declaration spanning those lines, the way
// a trailing comment on a one-line declaration does.
//
// commentsAttached lists the groups the parser did hang on something inside node,
// which win: a comment trailing a field on the same line as the brace is the
// field's, not the type's.
//
//declscope:package // collect.go asks this for funcs and for whole GenDecls
func (f *fileInfo) looseTrailingComments(fset *token.FileSet, node ast.Node, attached ...*ast.CommentGroup) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, pos := range []token.Pos{node.Pos(), node.End()} {
		g := f.commentsTrailingAt(fset, pos)
		if g == nil || slices.Contains(attached, g) || slices.Contains(out, g) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// commentsAttached returns the comment groups go/parser hung on a spec or on anything
// inside it, so that looseTrailingComments does not claim them for the enclosing
// declaration.
//
//declscope:package // collect.go asks this for every spec it visits
func commentsAttached(spec ast.Spec) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	switch spec := spec.(type) {
	case *ast.TypeSpec:
		out = append(out, spec.Doc, spec.Comment)
		var fields *ast.FieldList
		switch t := spec.Type.(type) {
		case *ast.StructType:
			fields = t.Fields
		case *ast.InterfaceType:
			fields = t.Methods
		}
		if fields != nil {
			for _, field := range fields.List {
				out = append(out, field.Doc, field.Comment)
			}
		}
	case *ast.ValueSpec:
		out = append(out, spec.Doc, spec.Comment)
	}
	return out
}
