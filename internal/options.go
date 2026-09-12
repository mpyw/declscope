//declscope:namespace analyzer

package internal

import (
	"regexp"
	"strings"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/scope"
)

// PrefixMode says when the namespace label is required on unexported
// package-level declarations.
type PrefixMode int

const (
	// PrefixOnDemand requires the label only in a package with more than one
	// namespace. In a package with one, there is no boundary for a label to
	// mark: every other rule is structurally inert there, since every
	// reference is already inside the single namespace, and a prefix repeated
	// on every declaration would distinguish nothing.
	PrefixOnDemand PrefixMode = iota
	// PrefixAlways requires the label unconditionally, so that a package
	// gaining its second namespace does not turn into a mass rename.
	PrefixAlways
	// PrefixNever disables the rule.
	PrefixNever
)

func (m PrefixMode) String() string {
	switch m {
	case PrefixOnDemand:
		return "ondemand"
	case PrefixAlways:
		return "true"
	case PrefixNever:
		return "false"
	default:
		return "unknown"
	}
}

// ParsePrefixMode reads the tri-state value of the rules.prefix setting, which
// YAML hands over as a bool for true and false and as a string for ondemand.
func ParsePrefixMode(value any) (PrefixMode, bool) {
	switch v := value.(type) {
	case bool:
		if v {
			return PrefixAlways, true
		}
		return PrefixNever, true
	case string:
		switch v {
		case "ondemand":
			return PrefixOnDemand, true
		case "true":
			return PrefixAlways, true
		case "false":
			return PrefixNever, true
		}
	}
	return 0, false
}

// required reports whether the label rule applies to a package with the given
// number of namespaces.
func (m PrefixMode) required(namespaces int) bool {
	switch m {
	case PrefixAlways:
		return true
	case PrefixOnDemand:
		return namespaces > 1
	default:
		return false
	}
}

// Options is the resolved configuration for a run.
type Options struct {
	// Exported is the scope of an exported identifier that carries no
	// directive.
	Exported scope.Scope
	// Unexported is the scope of an unexported identifier carrying no
	// directive.
	Unexported scope.Scope

	// CheckMembers bounds unexported methods and struct fields by the
	// namespace of the type they belong to, which is the encapsulation Go
	// itself cannot express.
	CheckMembers bool
	// CheckForeignMethods reports an unexported method declared on a type that
	// belongs to another namespace.
	CheckForeignMethods bool
	// Prefix says when an unexported package-level declaration must carry its
	// namespace as a label.
	Prefix PrefixMode

	// CheckDemote is the mirror of Prefix: where the label is not required, it
	// must not be present either. Together the two settle the spelling of
	// every unexported package-level name, in both directions. It is inert
	// under PrefixAlways, where the label is always required.
	CheckDemote bool

	// Exclude holds glob patterns matched against file paths.
	Exclude []string

	// BaselinePath is the baseline file to load, resolved relative to the
	// config file that named it. Empty means no baseline.
	BaselinePath string

	// Baseline suppresses violations that were already present when declscope
	// was adopted. It is nil when none is configured.
	Baseline *baseline.Set

	excludeRE []*regexp.Regexp
}

// DefaultOptions mirrors the rules stated in the README: exported is public,
// every other declaration is private to its namespace until a directive widens
// it, and the namespace prefix is a mandatory ownership label that grants
// nothing by itself.
func DefaultOptions() Options {
	return Options{
		Exported:            scope.Public,
		Unexported:          scope.FilePrivate,
		CheckMembers:        true,
		Prefix:              PrefixOnDemand,
		CheckDemote:         false,
		CheckForeignMethods: false,
	}
}

// Compile prepares the exclude patterns and loads the baseline. It must be
// called before use.
func (o *Options) Compile() error {
	o.excludeRE = o.excludeRE[:0]
	for _, pattern := range o.Exclude {
		re, err := compileGlob(pattern)
		if err != nil {
			return err
		}
		o.excludeRE = append(o.excludeRE, re)
	}
	if o.BaselinePath != "" {
		set, err := baseline.Load(o.BaselinePath)
		if err != nil {
			return err
		}
		o.Baseline = set
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

// resolve determines the scope of a package-level identifier.
//
// The namespace prefix plays no part in this. Encoding reach in the name would
// mean a prefix could not also be used simply to say which unit a declaration
// belongs to, and a prefix added for legibility would silently widen it.
// Reach is stated with a directive; the prefix only labels ownership.
func (o Options) resolve(name string, dir directive.Decl) scope.Scope {
	if dir.HasScope {
		return dir.Scope
	}
	if isExported(name) {
		return o.Exported
	}
	return o.Unexported
}

// resolveMember determines the scope of a method or struct field. Members are
// already namespaced by the type that owns them, so the namespace prefix rule
// deliberately does not apply: requiring User.userID would be exactly the
// stutter Go idiom avoids.
func (o Options) resolveMember(name string, dir directive.Decl) scope.Scope {
	if dir.HasScope {
		return dir.Scope
	}
	if isExported(name) {
		return scope.Public
	}
	return scope.FilePrivate
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
