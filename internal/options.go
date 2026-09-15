package internal

import (
	"regexp"
	"strings"

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

	// Exclude holds glob patterns matched against file paths.
	Exclude []string

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

	excludeRE []*regexp.Regexp
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

// Compile prepares the exclude patterns. It must be called before use.
//
// It does not load the baseline. Loading is the resolver's decision, since the
// same options serve both analysis, where the baseline suppresses, and
// regeneration, where the existing file must be ignored — otherwise one that
// fails to parse could never be regenerated.
func (o *Options) Compile() error {
	o.excludeRE = o.excludeRE[:0]
	for _, pattern := range o.Exclude {
		re, err := globFromOptions(pattern)
		if err != nil {
			return err
		}
		o.excludeRE = append(o.excludeRE, re)
	}
	return nil
}

// Excluded reports whether a file is outside the scope of the analysis.
func (o Options) Excluded(path string) bool {
	slashed := strings.ReplaceAll(path, "\\", "/")
	for _, re := range o.excludeRE {
		if re.MatchString(slashed) {
			return true
		}
	}
	return false
}

// globFromOptions translates a path glob into a regexp. ** matches across
// separators, * and ? do not.
func globFromOptions(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("(?:^|/)")
	for i := 0; i < len(pattern); {
		switch {
		case strings.HasPrefix(pattern[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 3
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i += 2
		case pattern[i] == '*':
			b.WriteString("[^/]*")
			i++
		case pattern[i] == '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			i++
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
