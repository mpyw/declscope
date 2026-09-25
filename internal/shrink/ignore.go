package shrink

import (
	"go/ast"
	"go/token"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/rule"
)

// silencedByIgnore reports whether an ignore naming overexported covers the
// candidate, and records every one that does. A bare ignore does not reach a
// module-wide rule: see rule.IsModuleWide.
//
//declscope:package // the core consults it before every report
func (r *run) silencedByIgnore(c *candidate) bool {
	hit := false
	for _, ig := range c.ignores {
		if slices.Contains(ig.Rules, rule.Overexported) {
			r.usedIgnores[ig.Pos] = true
			hit = true
		}
	}
	return hit
}

// unusedIgnores reports every ignore naming nothing but overexported, in the
// files of a judged package, that silenced nothing. One naming another rule
// as well is judged by neither side: the analyzer cannot see this rule, and
// this run cannot see the analyzer's.
//
// The report is the unused rule's, answered as the analyzer answers its own:
// by an ignore beside it naming unused, or by a file-level ignore covering
// unused, bare or named. The analyzer in turn never calls such an answer
// unused, since it cannot see the report it answers. siblings maps each
// ignore bound to a declaration to the ignores bound beside it.
//
//declscope:package // the core drains it after judging the package
func (r *run) unusedIgnores(p *packages.Package, siblings map[token.Pos][]directive.Ignore) []Finding {
	var findings []Finding
	for _, file := range p.Syntax {
		// A generated file declares no candidate, so an ignore there could
		// never silence one. The analyzer does not read it either.
		if ast.IsGenerated(file) {
			continue
		}
		fileIgnores := directive.ParseFile(file).Ignores
		for _, g := range file.Comments {
			group := directive.ParseDecl(g).Ignores
			for _, ig := range group {
				if r.usedIgnores[ig.Pos] || !ignoreJudged(ig) {
					continue
				}
				// An ignore bound to no declaration has only its own group.
				beside, ok := siblings[ig.Pos]
				if !ok {
					beside = group
				}
				if ignoreAnswered(ig, beside, fileIgnores) {
					continue
				}
				findings = append(findings, Finding{
					Pos:     r.mod.Fset.PositionFor(ig.Pos, false),
					Rule:    rule.Unused,
					Package: p.PkgPath,
				})
			}
		}
	}
	return findings
}

// ignoreJudged reports whether this run judges ig: it names module-wide
// rules and nothing else.
func ignoreJudged(ig directive.Ignore) bool {
	return len(ig.Rules) > 0 && !slices.ContainsFunc(ig.Rules, func(x rule.Rule) bool { return !rule.IsModuleWide(x) })
}

// ignoreAnswered reports whether another ignore answers ig's unused report:
// one beside it naming unused, or a file-level one covering it. No ignore
// answers its own report.
func ignoreAnswered(ig directive.Ignore, beside, fileIgnores []directive.Ignore) bool {
	for _, other := range beside {
		if other.Pos != ig.Pos && slices.Contains(other.Rules, rule.Unused) {
			return true
		}
	}
	for _, other := range fileIgnores {
		if other.Pos != ig.Pos && other.Covers(rule.Unused) {
			return true
		}
	}
	return false
}
