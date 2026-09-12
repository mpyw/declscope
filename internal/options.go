//declscope:namespace analyzer

package internal

import (
	"regexp"
	"strings"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/scope"
)

// Mode says when a naming rule applies: always, never, or only once a package
// has a second namespace. Both naming rules read one, so one settings block
// spells the same idea one way.
type Mode int

const (
	// Never disables the rule.
	Never Mode = iota

	// Always applies the rule to every package. For the label rule this
	// means a package gaining its second namespace is not a mass rename.
	Always

	// OnDemand applies the rule only to a package with more than one
	// namespace. In a package with one there is no boundary for a label to
	// mark: every other rule is structurally inert there, since every
	// reference is already inside the single namespace, and a prefix repeated
	// on every declaration would distinguish nothing.
	OnDemand
)

// String returns the spelling the settings use.
func (m Mode) String() string {
	switch m {
	case Never:
		return "never"
	case Always:
		return "always"
	case OnDemand:
		return "ondemand"
	default:
		return "unknown"
	}
}

// Applies reports whether the rule applies to a package with the given number
// of namespaces.
func (m Mode) Applies(namespaces int) bool {
	switch m {
	case Always:
		return true
	case OnDemand:
		return namespaces > 1
	default:
		return false
	}
}

// ModeSet is the values one setting accepts, in the order an error message
// names them. Each setting declares its own, so that a rejected value is
// answered with what that setting accepts rather than with everything Mode
// can hold.
type ModeSet []Mode

// Parse reads a setting's value: always, never or ondemand, and of those only
// the members of the set.
func (s ModeSet) Parse(value string) (Mode, bool) {
	for _, m := range s {
		if value == m.String() {
			return m, true
		}
	}
	return 0, false
}

// String lists the accepted spellings the way an error message names them:
// "always, never or ondemand".
func (s ModeSet) String() string {
	names := make([]string, len(s))
	for i, m := range s {
		names[i] = m.String()
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// Options is the resolved configuration for a run.
type Options struct {
	// Exported is the scope of an exported identifier that carries no
	// directive.
	Exported scope.Scope
	// Unexported is the scope of an unexported identifier carrying no
	// directive.
	Unexported scope.Scope

	// Qualify says when an unexported package-level declaration must carry its
	// namespace as a label.
	Qualify Mode

	// Unqualify is the mirror of Qualify: where the label is not required, it
	// must not be present either. Together the two settle the spelling of
	// every unexported package-level name, in both directions. It is inert
	// wherever Qualify applies, so it holds Always or Never.
	Unqualify Mode

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

// DefaultOptions mirrors the rules stated in the README: exported is public,
// every other declaration is private to its namespace until a directive widens
// it, and the namespace prefix is an ownership label, required once a package
// has a second namespace, that grants nothing by itself.
func DefaultOptions() Options {
	return Options{
		Exported:   scope.Public,
		Unexported: scope.FilePrivate,
		Qualify:    OnDemand,
		Unqualify:  Never,
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
		re, err := compileGlob(pattern)
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

// resolve determines the scope of a declaration, package-level or member
// alike.
//
// The namespace prefix plays no part in this. Encoding reach in the name would
// mean a prefix could not also be used simply to say which unit a declaration
// belongs to, and a prefix added for legibility would silently widen it.
// Reach is stated with a directive; the prefix only labels ownership.
//
// Members resolve the same way as everything else, so that defaults.exported
// and defaults.unexported govern every declaration in a package rather than
// half of them. What sets members apart is the boundary their scope is
// measured against — the namespace of their type, not of their file — and
// their exemption from the label rule, since a type already namespaces what
// it owns. Neither is a matter of scope, so neither belongs here.
func (o Options) resolve(name string, dir directive.Decl) scope.Scope {
	if dir.HasScope {
		return dir.Scope
	}
	if isExported(name) {
		return o.Exported
	}
	return o.Unexported
}

func isExported(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

// compileGlob translates a path glob into a regexp. ** matches across
// separators, * and ? do not.
func compileGlob(pattern string) (*regexp.Regexp, error) {
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
