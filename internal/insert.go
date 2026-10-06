package internal

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/scope"
)

// insertBook is insert.go's half of the collection, embedded there.
//
//declscope:shared // collection embeds it, and collection lives in the core
type insertBook struct {
	// sources holds each file a directive fix has read, by name. Every fix on
	// an indented declaration reads the file around it, and one file can hold
	// hundreds of them. A file that could not be read is held as nil.
	//
	//declscope:private // the type is widened only so the core can embed it
	sources map[string][]byte
}

// directiveInsertionFix inserts an explicit scope directive above the declaration.
//
//declscope:shared // finding.go offers it on a boundary finding
func (c *collection) directiveInsertionFix(pass *analysis.Pass, t *target, s scope.Scope) analysis.SuggestedFix {
	return analysis.SuggestedFix{
		Message:   fmt.Sprintf("add %s to %s", s.Directive(), t.name()),
		TextEdits: []analysis.TextEdit{c.directiveInsertion(pass, t, s, nil)},
	}
}

// directiveInsertion is the insertion itself.
//
// A directive only binds to a declaration when it sits on its own line above
// it, so a declaration that shares a line with something else — a field of a
// single-line struct, for instance — first has to be broken onto a line of its
// own. The formatter applied to the fixed file restores the indentation.
//
// doc is the comment the directive lands under, when the caller wants the
// house layout: a doc comment, a bare // line, then the directive. go/doc
// strips a directive from the rendered text either way, but the blank comment
// line is what keeps the two readable as separate things in the source. A doc
// comment that already ends in a directive or in a bare // needs no separator.
//
//declscope:shared // finding.go narrows a type's members with it, and surplus.go a declaration
func (c *collection) directiveInsertion(pass *analysis.Pass, t *target, s scope.Scope, doc *ast.CommentGroup) analysis.TextEdit {
	var text string
	if c.insertionAtLineStart(pass, t.anchor) {
		indent := strings.Repeat("\t", max(pass.Fset.PositionFor(t.anchor, false).Column-1, 0))
		text = s.Directive() + "\n" + indent
		if doc != nil && len(doc.List) > 0 && !insertionNeedsNoSeparator(doc) {
			text = "//\n" + indent + text
		}
	} else {
		text = "\n" + s.Directive() + "\n"
	}
	return analysis.TextEdit{Pos: t.anchor, End: t.anchor, NewText: []byte(text)}
}

// directiveLineBeforeInsertion is the shape go/ast recognizes as a directive: no
// space after the slashes, a lowercase word, and a colon. The three spellings
// without a colon are the ones cgo and the linker read.
var directiveLineBeforeInsertion = regexp.MustCompile(`^//(line |extern |export |[a-z0-9]+:[a-z0-9])`)

func insertionNeedsNoSeparator(doc *ast.CommentGroup) bool {
	last := doc.List[len(doc.List)-1].Text
	return last == "//" || directiveLineBeforeInsertion.MatchString(last)
}

// insertionAtLineStart reports whether pos is preceded on its line by nothing but
// whitespace. It fails safe: an unreadable file is treated as not starting a
// line, which yields an extra line break rather than a misplaced directive.
func (c *collection) insertionAtLineStart(pass *analysis.Pass, pos token.Pos) bool {
	// Unadjusted: the edit lands in the file on disk, and a //line directive
	// without a column leaves the adjusted column at 0.
	position := pass.Fset.PositionFor(pos, false)
	if position.Column <= 1 {
		return true
	}
	if pass.ReadFile == nil {
		return false
	}
	content := c.insertionSource(pass, position.Filename)
	if position.Offset > len(content) {
		return false
	}
	prefix := content[position.Offset-position.Column+1 : position.Offset]
	return strings.TrimLeft(string(prefix), " \t") == ""
}

// insertionSource reads a file once per pass. A read error is held as nil. Only
// a position past the start of its line reads the content, and every such
// position overruns nil, so the error still fails safe.
func (c *collection) insertionSource(pass *analysis.Pass, name string) []byte {
	if content, ok := c.sources[name]; ok {
		return content
	}
	if c.sources == nil {
		c.sources = make(map[string][]byte)
	}
	content, err := pass.ReadFile(name)
	if err != nil {
		content = nil
	}
	c.sources[name] = content
	return content
}
