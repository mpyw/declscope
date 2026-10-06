package internal

import (
	"fmt"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
)

// ignoreBook is ignore.go's half of the collection, embedded there. Its field
// belongs to this file's namespace, so only this file may reach it.
//
//declscope:shared // collection embeds it, and collection lives in the core
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
	//declscope:shared // collect.go names each target on it as it adds them
	decls []string
	//declscope:shared // collect.go marks it when the directive is the file's
	fileLevel bool
	used      bool
	// targets are the declarations the directive reaches, decls' objects. A
	// file the build excluded may cross into one of them by name.
	//
	//declscope:shared // collect.go adds each target as it names it
	targets []*target
	// siblings are the ignores parsed from the same comment group, which is
	// where a //declscope:ignore unused answering for this one is written.
	//
	//declscope:shared // collect.go records them as it parses the group
	siblings []directive.Ignore
}

// siteOfIgnore returns the accounting entry for ig, keyed by where it is written.
//
//declscope:shared // the collector registers every directive it parses
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
//declscope:shared // report.go consults it before every finding
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
//declscope:shared // surplus.go predicts what a boundary fix leaves behind
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
//declscope:shared // the collector and the scope accounting consult it too
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
//declscope:shared // report.go drains it after every finding has been seen
func (c *collection) reportUnusedIgnores(pass *analysis.Pass, opts Options) {
	if !opts.Unused.Reports() || c.unseen(pass).all {
		return
	}
	// An ignore naming a module-wide rule may be doing its job in a run this
	// pass is not, so it is left to declscope shrink to judge. So is one that
	// may be answering such an ignore's unused report there: beside it and
	// naming unused, or at the file level covering unused.
	moduleWideFiles := map[*fileInfo]bool{}
	for _, s := range c.ignores {
		if s.ig.NamesModuleWide() {
			moduleWideFiles[c.fileAt(pass, s.ig.Pos)] = true
		}
	}
	answersModuleWide := func(s *ignoreSite) bool {
		if slices.Contains(s.ig.Rules, rule.Unused) && slices.ContainsFunc(s.siblings, directive.Ignore.NamesModuleWide) {
			return true
		}
		return s.fileLevel && s.ig.Covers(rule.Unused) && moduleWideFiles[c.fileAt(pass, s.ig.Pos)]
	}
	// An ignore that may silence a crossing this pass cannot see has done its
	// job in the configuration that reads the crossing file.
	for _, s := range c.ignores {
		if !s.used && c.ignoreMayCrossUnseen(pass, s) {
			s.used = true
		}
	}
	sites := make([]*ignoreSite, 0, len(c.ignores))
	for _, s := range c.ignores {
		if !s.used && !s.ig.NamesModuleWide() && !answersModuleWide(s) {
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

// ignoreMayCrossUnseen reports whether an ignore of boundary may be silencing a
// crossing in a file the build excluded: that file writes the name of a
// declaration the ignore reaches. A configuration that reads the file reports
// the crossing, and the ignore answers it there, so it is not unused here.
// A file-level ignore reaches every declaration of its file.
//
// A field is crossed into by position too, by an unkeyed literal of its
// type, so the name of the type, or of an alias of it, counts for a field.
//
// The name is all an unseen file is read for, so a name it writes from the
// declaration's own namespace counts too. Such an ignore is still reported by
// the configuration that reads the file, which sees that nothing crosses.
// Every other rule is judged on the declaration, not on its uses, so an
// unseen file cannot make an ignore of one of them needed.
func (c *collection) ignoreMayCrossUnseen(pass *analysis.Pass, s *ignoreSite) bool {
	if !s.ig.Covers(rule.Boundary) {
		return false
	}
	names := c.unseen(pass).names
	if len(names) == 0 {
		return false
	}
	targets := s.targets
	if s.fileLevel {
		file := c.fileAt(pass, s.ig.Pos)
		targets = nil
		for _, t := range c.targets {
			if t.file == file {
				targets = append(targets, t)
			}
		}
	}
	return slices.ContainsFunc(targets, func(t *target) bool {
		return slices.ContainsFunc(c.ignoreCrossingNames(pass, t), func(n string) bool { return names[n] })
	})
}

// ignoreCrossingNames returns the names a file writes when it crosses into
// t: its own, and for a field the name of its type or of an alias of it. An
// unkeyed literal fills the fields by position and names only the type. The
// aliases are found once per pass.
func (c *collection) ignoreCrossingNames(pass *analysis.Pass, t *target) []string {
	out := []string{t.obj.Name()}
	v, ok := t.obj.(*types.Var)
	if !ok || !v.IsField() || t.ownerObj == nil {
		return out
	}
	if c.aliasNames == nil {
		c.aliasNames = map[types.Object][]string{}
		scope := pass.Pkg.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || !tn.IsAlias() {
				continue
			}
			if named, ok := types.Unalias(tn.Type()).(*types.Named); ok {
				c.aliasNames[named.Obj()] = append(c.aliasNames[named.Obj()], name)
			}
		}
	}
	return append(append(out, t.ownerObj.Name()), c.aliasNames[t.ownerObj]...)
}

// problemsLeftByIgnores drops the problems a file-level ignore stands
// down, marking the directive that did it used.
//
//declscope:shared // report.go and survey.go settle the problems no target holds
func (c *collection) problemsLeftByIgnores(pass *analysis.Pass) []directive.Problem {
	return slices.DeleteFunc(c.problems, func(p directive.Problem) bool {
		return c.ignoreSilencesFile(c.fileAt(pass, p.Pos), p)
	})
}
