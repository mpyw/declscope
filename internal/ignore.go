package internal

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
)

// ignoreBook is ignore.go's half of the collection, embedded there. Its field
// belongs to this file's namespace, so only this file may reach it.
//
//declscope:package // collection embeds it, and collection lives in the core
type ignoreBook struct {
	// ignores is every ignore directive in the package, keyed by where it is
	// written, so that one shared by several declarations is judged once.
	//
	//declscope:private // the type is widened only so the core can embed it
	ignores map[token.Pos]*ignoreSite
}

// ignoreSite is one physical ignore directive, however many declarations it
// reaches. A directive on a block is copied into every spec by Decl.Merge and
// one on `var a, b` is shared by both names, but it is still one comment, so it
// is judged once: unused only if it silencedByIgnore nothing for any of them. Judged
// per target, it would be reported unused whenever any sibling did not need
// it, and a wholly unused one would be reported once per sibling.
type ignoreSite struct {
	ig directive.Ignore
	// decls names the declarations the directive reaches, in source order,
	// for the report. It is empty for a file-level directive, and for one
	// carried only by declarations declscope does not check (init, _, an
	// embedded field), which is then unused by construction.
	//
	//declscope:package // collect.go names each target on it as it adds them
	decls []string
	//declscope:package // collect.go marks it when the directive is the file's
	fileLevel bool
	used      bool
	// siblings are the ignores parsed from the same comment group, which is
	// where a //declscope:ignore unused answering for this one is written.
	//
	//declscope:package // collect.go records them as it parses the group
	siblings []directive.Ignore
}

// siteOfIgnore returns the accounting entry for ig, keyed by where it is written.
//
//declscope:package // the collector registers every directive it parses
func (c *collection) siteOfIgnore(ig directive.Ignore) *ignoreSite {
	if c.ignores == nil {
		c.ignores = make(map[token.Pos]*ignoreSite)
	}
	s, ok := c.ignores[ig.Pos]
	if !ok {
		s = &ignoreSite{ig: ig}
		c.ignores[ig.Pos] = s
	}
	return s
}

// silencedByIgnore reports whether any ignore directive covering t silences r.
//
// A member inherits the directives written on the type that owns it, so the
// chain runs declaration, then owning type, then file. Every level is
// consulted rather than stopping at the first hit, and every directive that
// covers the rule is marked used, so overlapping directives at different
// levels do not make each other look unused.
//
//declscope:package // report.go consults it before every finding
func (c *collection) silencedByIgnore(t *target, r rule.Rule) bool {
	hit := c.ignored(t.dir.Ignores, r)
	// A member is written inside its type's declaration, so the type's ignores
	// contain it the way its scope directive does. A method with a receiver is
	// an ordinary top-level declaration and its type reaches neither: the
	// suppression chain and the scope chain walk the same levels, so that a
	// reader who learns one has learned both.
	if t.contained {
		if owner, ok := c.byObj[t.ownerObj]; ok && owner != t {
			hit = c.ignored(owner.dir.Ignores, r) || hit
		}
	}
	return c.ignored(t.file.ignores, r) || hit
}

// ignoreWouldSilence reports whether some ignore covering t silences r, on the
// same chain silencedByIgnore walks, without marking anything used.
//
// A fix that has to predict a finding the next run would make asks this. The
// finding does not exist in this run, so no directive has done any work yet,
// and marking one would hide the unused-ignore report this run owes.
//
//declscope:package // surplus.go predicts what a boundary fix leaves behind
func (c *collection) ignoreWouldSilence(t *target, r rule.Rule) bool {
	covers := func(ignores []directive.Ignore) bool {
		return slices.ContainsFunc(ignores, func(ig directive.Ignore) bool { return ig.Covers(r) })
	}
	if covers(t.dir.Ignores) || covers(t.file.ignores) {
		return true
	}
	if t.contained {
		if owner, ok := c.byObj[t.ownerObj]; ok && owner != t && covers(owner.dir.Ignores) {
			return true
		}
	}
	return false
}

// ignoreSilencesFile reports whether the file stands the problem's rule down
// for everything it holds, and marks the ignore that did it used.
//
// The marking is what keeps an ignore written for a directive problem from
// being reported as unused itself: it silences a report that is not attached to
// any declaration, so the per-target accounting never sees it work.
//
// An ignore never silences the report written at its own position, which is
// the report that it is unused. One that could would never be called unused,
// and that report exists to catch it.
//
//declscope:package // report.go consults it for problems, which no target holds
func (c *collection) ignoreSilencesFile(fi *fileInfo, p directive.Problem) bool {
	if fi == nil {
		return false
	}
	hit := false
	for _, ig := range fi.ignores {
		if ig.Pos != p.Pos && ig.Covers(p.Rule) {
			c.siteOfIgnore(ig).used = true
			hit = true
		}
	}
	return hit
}

// ignored reports whether any directive silences r, marking every directive
// that does as used. All of them are marked, not just the first, so that
// overlapping directives are not reported as unused.
//
//declscope:package // the collector and the scope accounting consult it too
func (c *collection) ignored(ignores []directive.Ignore, r rule.Rule) bool {
	hit := false
	for _, ig := range ignores {
		if ig.Covers(r) {
			c.siteOfIgnore(ig).used = true
			hit = true
		}
	}
	return hit
}

// reportUnusedIgnores reports every ignore directive that silenced nothing.
//
// Only a pass that sees every reference in the package can tell. The ordinary
// variant of a package with in-package _test.go files does not see the
// references those files make, so a directive needed only by a test would be
// reported unused there and reported necessary by the test variant, and the
// author could satisfy neither. That pass leaves the judgment to the test
// variant, on the same reasoning that keeps unmatched baseline entries
// unreported. Under -test (the default) the test variant runs and nothing is
// lost; with -test=false, a package with in-package tests gets no
// unused-ignore report at all, which is the only report that can be trusted.
//
// The report is the unused rule's, so another ignore covering that rule can
// answer it, and is then used: one written beside it on the same declaration
// when it names the rule, or one at the file level. No ignore answers its own,
// or it could never be called unused. The answers are taken over the ignores
// that silenced nothing before any answer counts, so the result does not
// depend on which is judged first. spec/unused_ignore.fsl is the model.
//
//declscope:package // report.go drains it after every finding has been seen
func (c *collection) reportUnusedIgnores(pass *analysis.Pass, opts Options) {
	if !opts.Unused.Reports() || c.unseen(pass).all {
		return
	}
	sites := make([]*ignoreSite, 0, len(c.ignores))
	for _, s := range c.ignores {
		if !s.used {
			sites = append(sites, s)
		}
	}
	slices.SortFunc(sites, func(a, b *ignoreSite) int { return comparePos(pass.Fset, a.ig.Pos, b.ig.Pos) })

	// Settle every answer before any report is made. An ignore answering one
	// of these has done a job, and its own report is not made. One answered by
	// a sibling is not reported at all; one answered at the file level is
	// reported and then silenced, as every problem is, so that the survey
	// counts it as ignored.
	bySibling := make(map[*ignoreSite]bool, len(sites))
	for _, s := range sites {
		for _, ig := range s.siblings {
			if ig.Pos != s.ig.Pos && slices.Contains(ig.Rules, rule.Unused) {
				bySibling[s] = true
				c.siteOfIgnore(ig).used = true
			}
		}
		// The report this would be, asked of the file before it exists, so
		// that the ignore answering it is marked used in time.
		c.ignoreSilencesFile(c.fileAt(pass, s.ig.Pos), directive.Problem{Pos: s.ig.Pos, Rule: rule.Unused})
	}
	for _, s := range sites {
		if s.used || bySibling[s] {
			continue
		}
		var msg string
		switch {
		case s.fileLevel:
			msg = fmt.Sprintf("unused file-level %s", s.ig)
		case len(s.decls) == 0:
			msg = fmt.Sprintf("unused %s: no checked declaration carries it", s.ig)
		default:
			msg = fmt.Sprintf("unused %s on %s", s.ig, strings.Join(s.decls, ", "))
		}
		c.problems = append(c.problems, directive.Problem{Pos: s.ig.Pos, Msg: msg, Rule: rule.Unused})
	}
}
