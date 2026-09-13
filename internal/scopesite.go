//declscope:namespace analyzer

package internal

import (
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/scope"
)

// scopeSite is one physical scope directive, however many declarations it
// reaches: a block's is copied into every spec, a type's reaches every field,
// and a file's reaches everything the file declares. It is judged once, like an
// ignore, and for the same reason — judged per target it would be reported
// whenever any sibling did not take it.
//
// The test is structural: is there anything in its reach that takes its scope
// from it. It deliberately does not ask whether the scope named differs from
// the one already in force. That question flips with a config key two
// directories up, so a single line of .declscope.yaml would report hundreds of
// directives in files nobody touched; and it would make //declscope:private,
// written to record that a declaration is private on purpose rather than by
// default, an error — which is the opposite of what a directive is for.
type scopeSite struct {
	dir directive.Decl
	// decls names the declarations in its reach, in source order, for the
	// report. Empty for a file-level directive.
	decls     []string
	fileLevel bool
	bound     bool
}

// scopeSite returns the accounting entry for a scope directive, keyed by where
// it is written.
func (c *collection) scopeSite(d directive.Decl) *scopeSite {
	s, ok := c.scopes[d.ScopePos]
	if !ok {
		s = &scopeSite{dir: d}
		c.scopes[d.ScopePos] = s
	}
	return s
}

// bind resolves a declaration's scope and records which directive supplied it.
//
// Only a subject can bind one. A declaration reachable from outside the package
// takes no scope at all, so a directive written on it is bound by nothing —
// unless it is a type, whose fields reach it through their own resolution and
// mark it there. That is the whole of the carve-out: an exported type with an
// unexported field has a subject beneath it, an exported func has none.
func (c *collection) bind(opts Options, subject bool, dir, container, file directive.Decl) scope.Scope {
	for _, d := range [...]directive.Decl{dir, container, file} {
		if !d.HasScope {
			continue
		}
		if subject {
			c.scopeSite(d).bound = true
		}
		return d.Scope
	}
	return opts.Unexported
}

// reportUnusedScopes reports every scope directive that bound nothing.
//
// Unlike an unused ignore this needs no complete view of the package's
// references: what a scope directive binds is decided by the declarations it
// reaches and the levels above them, never by who uses them. So it is reported
// in every variant, where an unused ignore defers to the one that sees every
// file.
func (c *collection) reportUnusedScopes(pass *analysis.Pass) {
	sites := make([]*scopeSite, 0, len(c.scopes))
	for _, s := range c.scopes {
		if !s.bound {
			sites = append(sites, s)
		}
	}
	slices.SortFunc(sites, func(a, b *scopeSite) int {
		return comparePos(pass.Fset, a.dir.ScopePos, b.dir.ScopePos)
	})
	for _, s := range sites {
		var msg string
		switch {
		case s.fileLevel:
			msg = "unused file-level " + s.dir.Scope.Directive()
		case len(s.decls) == 0:
			msg = "unused " + s.dir.Scope.Directive() + ": no checked declaration carries it"
		default:
			msg = "unused " + s.dir.Scope.Directive() + " on " + strings.Join(s.decls, ", ") +
				": nothing it reaches takes a scope"
		}
		c.problems = append(c.problems, directive.Problem{Pos: s.dir.ScopePos, Msg: msg})
	}
}
