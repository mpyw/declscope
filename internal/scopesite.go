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
//
//declscope:package // the collector registers and names the sites as it parses
type scopeSite struct {
	dir directive.Decl
	// decls names the declarations in its reach, in source order, for the
	// report. Empty for a file-level directive.
	decls     []string
	fileLevel bool
	bound     bool

	// shadowed records that something in its reach took a nearer directive's
	// scope instead. A block's directive that every spec overrides reaches no
	// declaration, exactly as one written on an init function does, and the
	// report has to separate the two: one line is redundant, the other is
	// written somewhere it can never bind.
	shadowed bool
}

// scopeSite returns the accounting entry for a scope directive, keyed by where
// it is written.
//
//declscope:package // the collector registers every directive it parses
func (c *collection) scopeSite(d directive.Decl) *scopeSite {
	s, ok := c.scopes[d.ScopePos]
	if !ok {
		s = &scopeSite{dir: d}
		c.scopes[d.ScopePos] = s
	}
	return s
}

// shadow records that a nearer directive supplied the scope of something outer
// reaches. Called where the two are merged, since after the merge only the
// winner's position survives.
//
//declscope:package // the collector merges directives, so it reports these
func (c *collection) shadow(outer, merged directive.Decl) {
	if outer.HasScope && merged.HasScope && merged.ScopePos != outer.ScopePos {
		c.scopeSite(outer).shadowed = true
	}
}

// bind resolves a declaration's scope and records which directive supplied it.
//
// The second result is the directive that supplied the scope, zero when the
// configured default did. A caller needs it to say which level decided, and to
// know whether inserting a directive on the declaration would overwrite an
// author's decision or merely state an exception to a default.
//
//declscope:package // the one scope resolution, shared with the collector
func (c *collection) bind(opts Options, name string, dir, container, file directive.Decl) (scope.Scope, directive.Decl, scopeLevel) {
	levels := []directive.Decl{dir, container, file}
	for i, d := range levels {
		if !d.HasScope {
			continue
		}
		if !c.inert(opts, name, d.Scope, levels[i+1:]) {
			c.scopeSite(d).bound = true
		}
		return d.Scope, d, scopeLevel(i + 1)
	}
	outer, _ := c.outerScope(opts, name, nil)
	return outer, directive.Decl{}, levelDefault
}

// outerScope is the scope a declaration would take from the levels outside the
// one being judged. The second result says whether that scope is the same under
// every configuration — it is not when it came from defaults.unexported, which
// is the whole reason the inert test can be asked at all.
//
// Exportedness decides the default and nothing else. An exported declaration is
// reached by every importer already, so the analysis has no line around it that
// it could also check; an author who states one is stating it, not guessing, and
// the directive binds.
func (c *collection) outerScope(opts Options, name string, rest []directive.Decl) (scope.Scope, bool) {
	for _, d := range rest {
		if d.HasScope {
			return d.Scope, true
		}
	}
	if isExported(name) {
		return scope.PackageInternal, true
	}
	return opts.Unexported, false
}

// inert reports whether stating a scope decides nothing, under every
// configuration: the declaration would have had that very scope anyway, and no
// setting could have made it otherwise.
//
// Asking only "is the name exported" would be wrong in both directions. It
// would call //declscope:package inert on an exported field whose type says
// private — where it is the only thing widening the field back, so a codebase
// could narrow an exported declaration and never widen it again without a
// permanent false report. And it would miss a directive that restates an
// enclosing one, which decides nothing for the same reason a redundant default
// does not: nothing about it could have gone another way.
func (c *collection) inert(opts Options, name string, stated scope.Scope, rest []directive.Decl) bool {
	outer, fixed := c.outerScope(opts, name, rest)
	return fixed && stated == outer
}

// reportUnusedScopes reports every scope directive that bound nothing.
//
// Unlike an unused ignore this needs no complete view of the package's
// references: what a scope directive binds is decided by the declarations it
// reaches and the levels above them, never by who uses them. So it is reported
// in every variant, where an unused ignore defers to the one that sees every
// file.
//
//declscope:package // report.go drains it after every finding has been seen
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
		case s.shadowed:
			msg = "unused " + s.dir.Scope.Directive() +
				": every declaration it reaches states its own scope"
		case len(s.decls) == 0:
			msg = "unused " + s.dir.Scope.Directive() + ": no checked declaration carries it"
		default:
			msg = "unused " + s.dir.Scope.Directive() + " on " + strings.Join(s.decls, ", ") +
				": nothing it reaches takes a scope"
		}
		c.problems = append(c.problems, directive.Problem{Pos: s.dir.ScopePos, Msg: msg})
	}
}
