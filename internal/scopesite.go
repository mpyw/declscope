//declscope:namespace analyzer

package internal

import (
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// scopeSite is one physical scope directive, however many declarations it
// reaches: a block's is copied into every spec, a type's reaches every field,
// and a file's reaches everything the file declares. It is judged once, like an
// ignore, and for the same reason — judged per target it would be reported
// whenever any sibling did not take it.
//
// A directive binds a declaration when the scope it names is one that
// declaration could not have had anyway — under ANY configuration. That
// quantifier is what keeps the test out of the trap a plain comparison falls
// into: comparing against defaults.unexported would flip every directive in a
// tree when one line of YAML changes, or when a config file appears two
// directories up, and would make recording a deliberate private an error.
//
// Quantified instead, the answer cannot depend on configuration at all:
//
//   - An UNEXPORTED declaration takes defaults.unexported, which may be either
//     scope, so neither //declscope:private nor //declscope:package is ever
//     inert on one. Recording an intent that matches today's default stays
//     legal, because tomorrow's default may differ.
//   - An EXPORTED declaration has no boundary unless a directive gives it one,
//     under every configuration. //declscope:package is the scope it already
//     has, so it is provably inert; //declscope:private narrows it, so it is
//     not.
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
// The second result is the directive that supplied the scope, zero when the
// configured default did. A caller needs it to say which level decided, and to
// know whether inserting a directive on the declaration would overwrite an
// author's decision or merely state an exception to a default.
func (c *collection) bind(opts Options, name string, dir, container, file directive.Decl) (scope.Scope, directive.Decl) {
	for _, d := range [...]directive.Decl{dir, container, file} {
		if !d.HasScope {
			continue
		}
		if !inert(name, d.Scope) {
			c.scopeSite(d).bound = true
		}
		return d.Scope, d
	}
	// Exportedness decides the default and nothing else. An exported declaration
	// is reached by every importer already, so the analysis has no line around it
	// that it could also check — but an author who states one is stating it, not
	// guessing, and the loop above binds.
	if isExported(name) {
		return scope.PackageInternal, directive.Decl{}
	}
	return opts.Unexported, directive.Decl{}
}

// inert reports whether stating a scope on a declaration decides nothing, under
// every configuration. See the comment on scopeSite.
func inert(name string, stated scope.Scope) bool {
	return isExported(name) && stated == scope.PackageInternal
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
		// A declaration-level ignore reaches this report through the directive
		// that carries it: the scope directive and the ignore were written on
		// the same declaration, so the author already answered.
		if !s.bound && !c.ignored(s.dir.Ignores, rule.Directive) {
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
