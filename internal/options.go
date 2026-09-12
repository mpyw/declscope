//declscope:namespace analyzer

package internal

import (
	"regexp"
	"strings"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/namespace"
	"github.com/mpyw/declscope/internal/scope"
)

// Options is the resolved configuration for a run.
type Options struct {
	// Exported is the scope of an exported identifier that carries no
	// directive.
	Exported scope.Scope
	// Unexported is the scope of an unexported identifier that carries neither
	// a directive nor its file's namespace prefix.
	Unexported scope.Scope
	// Prefixed is the scope of an unexported identifier that carries its
	// file's namespace prefix.
	Prefixed scope.Scope

	// CheckMembers bounds unexported methods and struct fields by the
	// namespace of the type they belong to, which is the encapsulation Go
	// itself cannot express.
	CheckMembers bool
	// CheckForeignMethods reports an unexported method declared on a type that
	// belongs to another namespace.
	CheckForeignMethods bool
	// CheckDemotion reports a namespace-prefixed identifier that is only ever
	// used inside its own namespace, so that names stay honest in both
	// directions.
	CheckDemotion bool

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
// unexported is private to its file, and a namespace prefix opts an
// identifier into package-wide visibility.
func DefaultOptions() Options {
	return Options{
		Exported:            scope.Public,
		Unexported:          scope.FilePrivate,
		Prefixed:            scope.PackageInternal,
		CheckMembers:        true,
		CheckForeignMethods: true,
		CheckDemotion:       false,
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
func (o Options) resolve(name, ns string, dir directive.Decl) scope.Scope {
	if dir.HasScope {
		return dir.Scope
	}
	if isExported(name) {
		return o.Exported
	}
	if namespace.HasPrefix(name, ns) {
		return o.Prefixed
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
