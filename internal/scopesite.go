package internal

import (
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// scopesiteBook is scopesite.go's half of the collection, embedded there.
//
//declscope:package // collection embeds it, and collection lives in the core
type scopesiteBook struct {
	// scopes is the accounting for scope directives: one entry per physical
	// comment, marked when something in its reach takes its scope.
	//
	//declscope:private // the type is widened only so the core can embed it
	scopes map[token.Pos]*scopeSite
}

// scopeSite is one physical scope directive, however many declarations it
// reaches: a block's is copied into every spec, a type's reaches every field,
// and a file's reaches everything the file declares. It is judged once, like an
// ignore, and for the same reason — judged per target it would be reported
// whenever any sibling did not take it.
//
// A directive binds a declaration when the scope it names is one that
// declaration could not have had anyway — under ANY configuration. That
// quantifier is what rules.unused: loose, the default, judges by. Comparing
// against defaults.unexported instead changes the judgment of every directive
// in a tree when one line of YAML changes, or when a config file appears two
// directories up, and reports a directive recording a deliberate private.
// rules.unused: strict is the opt-in that makes that comparison
// (redundantAtScopeSite) and accepts the trade.
//
// Quantified, the answer cannot depend on configuration at all:
//
//   - An UNEXPORTED declaration takes defaults.unexported, which may be either
//     scope, so neither //declscope:private nor //declscope:package is ever
//     inert on one. Under loose, recording an intent that matches today's
//     default is not reported, because tomorrow's default may differ.
//   - An EXPORTED declaration has no boundary unless a directive gives it one,
//     under every configuration. //declscope:package is the scope it already
//     has, so it is provably inert; //declscope:private narrows it, so it is
//     not.
type scopeSite struct {
	dir directive.Decl
	// decls names the declarations in its reach, in source order, for the
	// report. Empty for a file-level directive.
	//
	//declscope:package // collect.go names each target on it as it adds them
	decls []string
	//declscope:package // collect.go marks it when the directive is the file's
	fileLevel bool
	bound     bool

	// shadowed records that something in its reach took a nearer directive's
	// scope instead. A block's directive that every spec overrides reaches no
	// declaration, exactly as one written on an init function does, and the
	// report has to separate the two: one line is redundant, the other is
	// written somewhere it can never bindAtScopeSite.
	shadowed bool

	// taken records that something in its reach took this directive's scope,
	// and overridden that something took the other scope from a nearer one.
	// They keep a report from saying a declaration has a scope it does not.
	// A file-level report names no declaration, so it reads these alone.
	taken      bool
	overridden bool

	// reached and needed are the strict judgment, made over everything in the
	// directive's reach, shadowed or not. reached records that anything was
	// judged; needed that something would take another scope without the
	// directive, under the configuration in force. See redundantAtScopeSite.
	reached bool
	needed  bool

	// outers are the directives that would supply the scope of what takes it
	// from this one, were this one deleted. restatesOuter records that one of
	// them names the same scope, which a deletion would hand the declaration
	// to as a new dependent.
	outers        []token.Pos
	restatesOuter bool
}

// scopeSite returns the accounting entry for a scope directive, keyed by where
// it is written.
//
//declscope:package // the collector registers every directive it parses
func (c *collection) scopeSite(d directive.Decl) *scopeSite {
	if c.scopes == nil {
		c.scopes = make(map[token.Pos]*scopeSite)
	}
	s, ok := c.scopes[d.ScopePos]
	if !ok {
		s = &scopeSite{dir: d}
		c.scopes[d.ScopePos] = s
	}
	return s
}

// shadowedAtScopeSite records that a nearer directive supplied the scope of something outer
// reaches. Called where the two are merged, since after the merge only the
// winner's position survives.
//
//declscope:package // the collector merges directives, so it reports these
func (c *collection) shadowedAtScopeSite(outer, merged directive.Decl) {
	if outer.HasScope && merged.HasScope && merged.ScopePos != outer.ScopePos {
		c.scopeSite(outer).shadowed = true
	}
}

// scopesiteLevel names which level supplied a scope. A diagnostic that
// inferred it from the kind instead would tell a reader to look for a comment
// that is not there: a field takes its type's directive and its file's alike,
// and only the level knows which one decided.
//
//declscope:package // subject.go's target records it in boundAt
type scopesiteLevel int

//declscope:package // report.go words each boundary finding by the level
const (
	scopesiteLevelDefault scopesiteLevel = iota
	scopesiteLevelDecl
	scopesiteLevelContainer
	scopesiteLevelFile
)

// bindAtScopeSite resolves a declaration's scope and records which directive supplied it.
//
// The second result is the directive that supplied the scope, zero when the
// configured default did. A caller needs it to say which level decided, and to
// know whether inserting a directive on the declaration would overwrite an
// author's decision or merely state an exception to a default.
//
//declscope:package // the one scope resolution, shared with the collector
func (c *collection) bindAtScopeSite(opts Options, name string, dir, container, file directive.Decl) (scope.Scope, directive.Decl, scopesiteLevel, bool) {
	// The chain as written: a merge hides the block's directive beneath a spec
	// that states its own, and both judgments need it back. Without it, a
	// spec's //declscope:private under a //declscope:package block would be
	// judged against the file, and called inert where deleting it widens the
	// spec. Resolution is unaffected: a hidden directive is never the first.
	chain := []directive.Decl{dir, dir.BeneathScope(), container, container.BeneathScope(), file}
	levels := []scopesiteLevel{scopesiteLevelDecl, scopesiteLevelDecl, scopesiteLevelContainer, scopesiteLevelContainer, scopesiteLevelFile}
	c.redundantAtScopeSite(opts, name, chain)
	for i, d := range chain {
		if !d.HasScope {
			continue
		}
		decided := !isInertScopeSite(opts, name, d.Scope, chain[i+1:])
		if decided {
			c.scopeSite(d).bound = true
		}
		return d.Scope, d, levels[i], decided
	}
	outer, _ := outerScopeOfScopeSite(opts, name, nil)
	return outer, directive.Decl{}, scopesiteLevelDefault, false
}

// outerScopeOfScopeSite is the scope a declaration would take from the levels outside the
// one being judged. The second result says whether that scope is the same under
// every configuration — it is not when it came from defaults.unexported, which
// is the whole reason the inert test can be asked at all.
//
// Exportedness decides the default and nothing else. An exported declaration is
// reached by every importer already, so the analysis has no line around it that
// it could also check; an author who states one is stating it, not guessing, and
// the directive binds.
func outerScopeOfScopeSite(opts Options, name string, rest []directive.Decl) (scope.Scope, bool) {
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

// isInertScopeSite reports whether stating a scope decides nothing, under every
// configuration: the declaration would have had that very scope anyway, and no
// setting could have made it otherwise.
//
// Asking only "is the name exported" would be wrong in both directions. It
// would call //declscope:package inert on an exported field whose type says
// private — where it is the only thing surplus the field back, so a codebase
// could narrow an exported declaration and never widen it again without a
// permanent false report. And it would miss a directive that restates an
// enclosing one, which decides nothing for the same reason a redundant default
// does not: nothing about it could have gone another way.
func isInertScopeSite(opts Options, name string, stated scope.Scope, rest []directive.Decl) bool {
	outer, fixed := outerScopeOfScopeSite(opts, name, rest)
	return fixed && stated == outer
}

// redundantAtScopeSite makes the strict judgment for one declaration: each
// directive in its chain, the one that decided and every one it shadows, is
// asked whether it names the scope the declaration would take without it,
// under the configuration in force. The resolution is bindAtScopeSite's: the
// first directive in the chain decides, and outerScopeOfScopeSite answers what
// would decide in its absence.
//
// The chain is the one bindAtScopeSite walks, with what a merge hid put back:
// a block's directive beneath a spec that states its own, for the spec and for
// a field of the type the spec declares. Without it, a spec's
// //declscope:private under a //declscope:package block would be judged
// against the file, and deleting it would widen the spec.
//
// A shadowed declaration is judged as well, against the levels beyond the
// directive. The nearer directive that shadows it is judged against this one's
// scope today and against those levels once this one is deleted, so the two
// must agree for the deletion to leave that judgment where it was.
func (c *collection) redundantAtScopeSite(opts Options, name string, chain []directive.Decl) {
	var first *directive.Decl
	for i, d := range chain {
		if !d.HasScope {
			continue
		}
		s := c.scopeSite(d)
		s.reached = true
		outer, _ := outerScopeOfScopeSite(opts, name, chain[i+1:])
		if d.Scope != outer {
			s.needed = true
		}
		if first != nil {
			s.shadowed = true
			s.overridden = s.overridden || first.Scope != d.Scope
			continue
		}
		first = &chain[i]
		s.taken = true
		if next, ok := nextScopeSite(chain[i+1:]); ok {
			s.outers = append(s.outers, next.ScopePos)
			s.restatesOuter = s.restatesOuter || next.Scope == d.Scope
		}
	}
}

// nextScopeSite is the first directive in rest that states a scope.
func nextScopeSite(rest []directive.Decl) (directive.Decl, bool) {
	for _, d := range rest {
		if d.HasScope {
			return d, true
		}
	}
	return directive.Decl{}, false
}

// redundant reports whether the strict judgment found the directive deciding
// nothing today: something was judged, and nothing would take another scope
// without it.
func (s *scopeSite) redundant() bool { return s.reached && !s.needed }

// reportUnusedScopeSites reports every scope directive that bound nothing.
//
// Unlike an unused ignore this needs no complete view of the package's
// references: what a scope directive binds is decided by the declarations it
// reaches and the levels above them, never by who uses them. So it is reported
// in every variant, where an unused ignore defers to the one that sees every
// file.
//
// Under rules.unused: strict it also reports a directive that is redundant
// today (redundantAtScopeSite), and offers to delete it. Under off it reports
// nothing, and so offers nothing. widened names the types a boundary fix in
// this run widens; nil when no fix is made.
//
//declscope:package // report.go drains it after every finding has been seen
func (c *collection) reportUnusedScopeSites(pass *analysis.Pass, opts Options, widened map[types.Object]bool) {
	if !opts.Unused.Reports() {
		return
	}
	strict := opts.Unused.ReportsRedundant()
	unused := func(s *scopeSite) bool { return !s.bound || strict && s.redundant() }
	sites := make([]*scopeSite, 0, len(c.scopes))
	for _, s := range c.scopes {
		if !unused(s) {
			continue
		}
		// A declaration-level ignore reaches this report through the directive
		// that carries it: the scope directive and the ignore were written on
		// the same declaration, so the author already answered.
		if c.ignored(s.dir.Ignores, rule.Unused) {
			continue
		}
		sites = append(sites, s)
	}
	slices.SortFunc(sites, func(a, b *scopeSite) int {
		return comparePos(pass.Fset, a.dir.ScopePos, b.dir.ScopePos)
	})
	var removals *scopesiteRemovals
	if strict {
		removals = &scopesiteRemovals{c: c, pass: pass, opts: opts, widened: widened, unused: unused}
	}
	for _, s := range sites {
		redundant := strict && s.redundant()
		var fixes []analysis.SuggestedFix
		if redundant {
			if fix, ok := removals.fix(s); ok {
				fixes = []analysis.SuggestedFix{fix}
			}
		}
		msg := messageOfScopeSite(s, redundant)
		c.problems = append(c.problems, directive.Problem{Pos: s.dir.ScopePos, Msg: msg, Rule: rule.Unused, Fixes: fixes})
	}
}

// messageOfScopeSite words the report on an unused scope directive, under
// either mode. redundant says that strict reports it, where loose may not.
//
// Each reason is chosen to be true of everything the directive reaches. A
// declaration that states its own scope is never said to have the directive's:
// it may name the other scope. Only the declarations that take the directive's
// scope are named, and a file-level report, which names none, says which of
// the two kinds it reached.
//
// A strict reason says what the declarations already have and not where it
// comes from: deleting an outer directive in the same run can move the source,
// and a report that survives the run must read the same after it.
func messageOfScopeSite(s *scopeSite, redundant bool) string {
	d := s.dir.Scope.Directive()
	has := "already has " + strings.TrimPrefix(d, "//declscope:") + " scope"
	if s.fileLevel {
		const nearer = "takes a nearer directive's scope"
		switch {
		case !redundant:
			return "unused file-level " + d
		case !s.overridden:
			return "unused file-level " + d + ": every declaration it reaches " + has
		case !s.taken:
			return "unused file-level " + d + ": every declaration it reaches " + nearer
		default:
			return "unused file-level " + d + ": every declaration it reaches " + nearer + " or " + has
		}
	}
	if len(s.decls) == 0 {
		// No declaration is named, which is not the same as none taking the
		// scope: a field of a type that declares no name takes it through its
		// type, and is never named. So taken, not the names, decides whether
		// "states its own scope" is true of everything reached.
		switch {
		case !s.taken && !s.shadowed:
			return "unused " + d + ": no checked declaration carries it"
		// A spec restating its block, or a field of an unnamed type.
		case redundant && !s.overridden:
			return "unused " + d + ": every declaration it reaches " + has
		case !s.taken:
			return "unused " + d + ": every declaration it reaches states its own scope"
		case !redundant:
			return "unused " + d + ": nothing it reaches takes a scope"
		default:
			return "unused " + d + ": every declaration it reaches states its own scope or " + has
		}
	}
	switch {
	case !redundant:
		return "unused " + d + " on " + strings.Join(s.decls, ", ") + ": nothing it reaches takes a scope"
	case len(s.decls) == 1:
		return "unused " + d + " on " + s.decls[0] + ": it " + has
	default:
		return "unused " + d + " on " + strings.Join(s.decls, ", ") + ": each " + has
	}
}

// rewordedAtScopeSite reports whether narrowing the declarations in narrowed
// would change the unused report on d, a directive that binds nothing and so
// is reported whenever the rule is on.
//
// A narrowed declaration that took d's scope states //declscope:private
// instead: it leaves the names on the report and joins what overrides d. What
// d decides does not move, since each narrowed declaration is judged against
// the same levels beyond it, so the report is only reworded, never dropped.
//
//declscope:package // surplus.go withholds a narrowing that would reword it
func (c *collection) rewordedAtScopeSite(opts Options, d directive.Decl, narrowed map[*target]bool) bool {
	if !opts.Unused.Reports() {
		return false
	}
	s := c.scopeSite(d)
	after := *s
	after.decls, after.taken = nil, false
	for _, t := range c.targets {
		switch {
		case t.boundBy.ScopePos != d.ScopePos:
		case narrowed[t]:
			after.shadowed, after.overridden = true, true
		default:
			after.taken = true
			if t.dir.ScopePos == d.ScopePos {
				after.decls = append(after.decls, t.name())
			}
		}
	}
	redundant := opts.Unused.ReportsRedundant() && s.redundant()
	return messageOfScopeSite(s, redundant) != messageOfScopeSite(&after, redundant)
}

// scopesiteRemovals decides which strict reports carry the fix that deletes
// the directive, once per run, and builds it.
//
// Deleting a redundant directive leaves every declaration's scope where it was
// (redundantAtScopeSite), and deleting several in one run does too: each names
// the scope the levels beyond it give, and those levels keep giving it. What
// can still move is another report, and the fix is withheld wherever one
// would, since -fix must not produce a diagnostic the run did not start with:
//
//   - A declaration taking its scope from the directive is used from another
//     namespace. The boundary message names the level that decided, and that
//     level moves.
//   - It is a field whose type a boundary fix in this run widens. The field
//     would take the type's new directive instead of the level beyond this one.
//   - The directive is //declscope:package and surplus reads it: a declaration
//     would join an enclosing directive's dependents, or an ignore answering
//     the surplus rule covers it and could be left answering nothing.
//   - The level beyond it is a directive that is reported and keeps its
//     report. The declaration would join that one, and its report would
//     change or vanish under an ignore still answering it.
//   - The pass does not read every file: the ordinary variant of a package
//     with in-package tests, or a file the build excludes. A crossing may sit
//     where it cannot look, and the driver applies what any variant offers.
type scopesiteRemovals struct {
	c       *collection
	pass    *analysis.Pass
	opts    Options
	widened map[types.Object]bool
	unused  func(*scopeSite) bool
	takers  map[token.Pos][]*target
	memo    map[*scopeSite]bool
}

// fix returns the deletion for a strict report, when it may be offered.
func (r *scopesiteRemovals) fix(s *scopeSite) (analysis.SuggestedFix, bool) {
	if !r.fixable(s) {
		return analysis.SuggestedFix{}, false
	}
	edit, ok := scopesiteRemoval(r.pass, s.dir.ScopePos)
	if !ok {
		return analysis.SuggestedFix{}, false
	}
	return analysis.SuggestedFix{
		Message:   "remove " + s.dir.Scope.Directive(),
		TextEdits: []analysis.TextEdit{edit},
	}, true
}

// fixable is the decision, memoized: a directive's answer can depend on the
// one beyond it, which is asked again for every directive it encloses.
func (r *scopesiteRemovals) fixable(s *scopeSite) bool {
	if r.memo == nil {
		r.memo = make(map[*scopeSite]bool)
		r.takers = make(map[token.Pos][]*target)
		for _, t := range r.c.targets {
			if t.boundBy.HasScope {
				r.takers[t.boundBy.ScopePos] = append(r.takers[t.boundBy.ScopePos], t)
			}
		}
	}
	if ok, seen := r.memo[s]; seen {
		return ok
	}
	// A cycle is impossible, since an outer directive is always further out,
	// but a false entry first makes one terminate rather than recurse.
	r.memo[s] = false
	ok := r.decide(s)
	r.memo[s] = ok
	return ok
}

func (r *scopesiteRemovals) decide(s *scopeSite) bool {
	// Only an outer directive can fail this: loose may be what reports it.
	// One an ignore answers is judged like any other. If it is redundant it
	// stays redundant, and answered, once it gains a declaration, so nothing
	// shows; if it is not, this withholds.
	if !s.redundant() {
		return false
	}
	// A pass that does not read every file cannot see every crossing, and the
	// driver applies what any variant offers. The test variant decides.
	if r.c.unseen(r.pass).all {
		return false
	}
	surplusReads := s.dir.Scope == scope.PackageInternal && r.opts.Surplus.Reports()
	if surplusReads && s.restatesOuter {
		return false
	}
	for _, t := range r.takers[s.dir.ScopePos] {
		if r.opts.Boundary.Reports() && t.scope == scope.Private && r.crosses(t) {
			return false
		}
		if t.contained && r.widened[t.ownerObj] {
			return false
		}
		if surplusReads && r.c.ignoreWouldSilence(t, rule.Surplus) {
			return false
		}
	}
	for _, pos := range s.outers {
		outer, ok := r.c.scopes[pos]
		if ok && r.unused(outer) && !r.fixable(outer) {
			return false
		}
	}
	return true
}

// crosses reports whether another namespace spells t, the test the boundary
// rule reports on. A file the build excludes may spell it from anywhere, and
// is read for its names alone, so a name it writes counts as a crossing.
func (r *scopesiteRemovals) crosses(t *target) bool {
	if r.c.unseen(r.pass).names[t.obj.Name()] {
		return true
	}
	for _, ref := range r.c.refs[t.obj] {
		if ref.file.key() != t.file.key() {
			return true
		}
	}
	return false
}

// scopesiteRemoval deletes the directive comment at pos: its whole line when it
// stands alone, or the comment and the space before it when it trails code.
//
// A doc comment is left without the bare // that separated it from the
// directive, since the separator would otherwise end the comment. A directive
// standing between blank lines, as a file-level one often does, takes one of
// them along, so that no double blank line is left.
//
// It fails safe: an unreadable file yields no fix.
func scopesiteRemoval(pass *analysis.Pass, pos token.Pos) (analysis.TextEdit, bool) {
	if pass.ReadFile == nil {
		return analysis.TextEdit{}, false
	}
	tf := pass.Fset.File(pos)
	if tf == nil {
		return analysis.TextEdit{}, false
	}
	content, err := pass.ReadFile(tf.Name())
	if err != nil {
		return analysis.TextEdit{}, false
	}
	off := tf.Offset(pos)
	if off > len(content) {
		return analysis.TextEdit{}, false
	}
	lineStart := strings.LastIndexByte(string(content[:off]), '\n') + 1
	lineEnd := len(content)
	if i := strings.IndexByte(string(content[off:]), '\n'); i >= 0 {
		lineEnd = off + i
	}
	if lead := strings.TrimRight(string(content[lineStart:off]), " \t"); lead != "" {
		// Trailing a declaration: the comment goes, the code and the line
		// ending stay, a CRLF one included.
		end := lineEnd
		if end > off && content[end-1] == '\r' {
			end--
		}
		return analysis.TextEdit{Pos: tf.Pos(lineStart + len(lead)), End: tf.Pos(end)}, true
	}
	start, end := lineStart, min(lineEnd+1, len(content))
	prevStart, prev := scopesiteLine(content, lineStart, -1)
	_, next := scopesiteLine(content, end, +1)
	switch {
	case prev == "//" && !strings.HasPrefix(next, "//"):
		start = prevStart
	case prev == "" && next == "" && end < len(content):
		// Between blank lines, or at the top of the file above one.
		if nl := strings.IndexByte(string(content[end:]), '\n'); nl >= 0 {
			end += nl + 1
		}
	}
	return analysis.TextEdit{Pos: tf.Pos(start), End: tf.Pos(end)}, true
}

// scopesiteLine returns the line before the one starting at off (dir -1), or
// the one starting at off (dir +1), trimmed of spaces, with the offset it
// starts at. At either edge of the file it returns "".
func scopesiteLine(content []byte, off, dir int) (int, string) {
	if dir < 0 {
		if off == 0 {
			return 0, ""
		}
		start := strings.LastIndexByte(string(content[:off-1]), '\n') + 1
		return start, strings.TrimSpace(string(content[start : off-1]))
	}
	end := len(content)
	if i := strings.IndexByte(string(content[off:]), '\n'); i >= 0 {
		end = off + i
	}
	return off, strings.TrimSpace(string(content[off:end]))
}
