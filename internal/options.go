package internal

import (
	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/scope"
)

type Options struct {
	// Unexported is the scope of a declaration that states none of its own and
	// inherits none. There is no Exported counterpart: what is reachable from
	// outside the package is not declscope's subject, and a key that claimed
	// otherwise would promise an enforcement the analysis cannot perform.
	Unexported scope.Scope

	// Qualify says when a package-level declaration must carry its namespace
	// somewhere in its name.
	Qualify Mode

	// Vocabulary lists, per namespace, extra words that carry the namespace as
	// its own spelling would: irregular inflections and domain synonyms that
	// no generated form reaches (mouse: wheel, index: indices). A word is
	// matched exactly the way the namespace is — starting at a word boundary,
	// with the right edge free — so it is a spelling, never a scope.
	Vocabulary map[string][]string

	// AllowBoundary turns the boundary rule off. The rule reports a private
	// declaration used from outside its namespace, and is the one this tool
	// exists for, so switching it off leaves only the naming rule.
	//
	// It is here for the repository that wants the ownership mark in a name
	// without the scope behind it. Reach stays unchecked, //declscope:package
	// stops meaning anything, and surplus keeps auditing directives that no
	// longer do a job -- set allowSurplus alongside it.
	//
	// This is not the way to adopt declscope gradually. A baseline records
	// what a codebase already has and still reports what is new, which a
	// switch cannot do.
	AllowBoundary bool

	// AllowSurplus turns the surplus rule off. The rule reports a
	// //declscope:package directive when no use from another namespace is
	// visible to declscope, and is on by default: a directive nobody needed is
	// a thing the author would want told.
	//
	// The polarity is stated rather than inverted in the reader's head. A key
	// named surplus would have read as "surplus: yes please", which is the
	// opposite of what setting it to true would do.
	//
	// The rule never has a fix. It concludes from an absence, so every case it
	// cannot see is one where the directive stays and the advice would be to
	// delete it.
	AllowSurplus bool

	// NameExported widens the naming rule to exported declarations. Inside
	// the package an exported name is read as bare as any other, so the package
	// qualifier that explains an external use is absent exactly where the
	// namespace mark is wanted. The violation is reported. The rename is never
	// offered, since the uses outside the package cannot be seen.
	NameExported bool

	// Only narrows the analysis, one group per config file that stated it. A
	// file is read when it matches at least one pattern in EVERY group, so the
	// groups intersect and a nested config can narrow further but never widen.
	// No groups is not "match nothing" but "no restriction", which is why a
	// repository with no config is read whole.
	//
	// The groups are separate rather than one flat list because each config
	// file anchors its own patterns: "gen/**" in the root and "gen/**" in a
	// nested file name different directories, and flattening them would lose
	// which is which.
	Only [][]FilterPattern

	// Omit takes files back out, and the chain unions rather than intersecting:
	// a file matching any pattern from any level is not read. An omit written
	// at the root therefore holds everywhere below it. Omit is the stronger of
	// the two, and applies whether or not Only let the file through.
	Omit []FilterPattern

	// BaselinePath is the baseline file that applies, resolved relative to
	// the config file that named it or found by the default-named lookup.
	// Empty means no baseline.
	BaselinePath string

	// Baseline suppresses violations that were already present when declscope
	// was adopted. It is nil when none is configured, and also while a
	// baseline is being regenerated: config.Resolve loads it, config.
	// ResolveForBaseline deliberately does not, so that a baseline which fails
	// to parse cannot block its own regeneration.
	Baseline *baseline.Set

	onlyRE [][]filterMatcher
	omitRE []filterMatcher
}

// DefaultOptions mirrors the rules stated in the README: every declaration in
// the subject is private to its namespace until something widens it. The
// naming rule is off by default — measured over the repositories this tool
// was built against, packages with zero boundary violations still drew dozens
// of naming ones, and whether a name reads well with its namespace in it
// depends on the part of speech of the file name, which the tool cannot see.
// A codebase that wants the convention states rules.naming.qualify itself.
func DefaultOptions() Options {
	return Options{
		Unexported: scope.Private,
		Qualify:    ModeNever,
	}
}

// Compile prepares the filter patterns. It must be called before use.
//
// It does not load the baseline. Loading is the resolver's decision, since the
// same options serve both analysis, where the baseline suppresses, and
// regeneration, where the existing file must be ignored — otherwise one that
// fails to parse could never be regenerated.
func (o *Options) Compile() error {
	compile := func(patterns []FilterPattern) ([]filterMatcher, error) {
		out := make([]filterMatcher, 0, len(patterns))
		for _, p := range patterns {
			m, err := compileFilter(p.Pattern, filterBases(p.Base))
			if err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		return out, nil
	}
	onlyRE := make([][]filterMatcher, 0, len(o.Only))
	for _, group := range o.Only {
		ms, err := compile(group)
		if err != nil {
			return err
		}
		onlyRE = append(onlyRE, ms)
	}
	omitRE, err := compile(o.Omit)
	if err != nil {
		return err
	}
	o.onlyRE, o.omitRE = onlyRE, omitRE
	return nil
}

// Skips reports whether a file is outside the scope of the analysis.
//
// Within one level the two tests are applied in the order the config reads,
// and the order is not a choice: both lists ask about one path, so narrowing
// before subtracting and subtracting before narrowing name the same set.
//
// Across levels they compose differently. Every only group must admit the
// file, and any omit pattern from any level rejects it, so a config file can
// only ever shrink what is read.
func (o Options) Skips(path string) bool {
	for _, group := range o.onlyRE {
		if !filterMatches(group, path) {
			return true
		}
	}
	return filterMatches(o.omitRE, path)
}

// NearestOnly reports the only group of the config file closest to the
// package, and whether there is one. It is what tells a config whose only
// matched nothing from one whose only was cancelled by a group above it.
func (o Options) NearestOnly() (FilterPattern, bool) {
	if len(o.Only) == 0 {
		return FilterPattern{}, false
	}
	group := o.Only[len(o.Only)-1]
	if len(group) == 0 {
		return FilterPattern{}, false
	}
	return group[0], true
}

// NearestOnlyAdmits reports whether the nearest only group would read the file
// on its own, with the groups above it and every omit set aside.
func (o Options) NearestOnlyAdmits(path string) bool {
	if len(o.onlyRE) == 0 {
		return false
	}
	return filterMatches(o.onlyRE[len(o.onlyRE)-1], path)
}
