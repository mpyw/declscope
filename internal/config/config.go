// Package config loads declscope's YAML configuration.
//
// A config file is optional. When present it is named .declscope.yaml (or
// .yml) and is looked up from the directory of the package being analyzed
// upwards, so that a subtree can relax the rules without the rest of the
// module following suit.
//
//	defaults:
//	  unexported: private
//
//	rules:
//	  naming:
//	    qualify: ondemand    # always | never | ondemand (only once a package has two namespaces)
//	    vocabulary:          # per-namespace words that carry the namespace
//	      mouse: [wheel]
//	  boundary: on          # off | on (off stops checking reach, leaving only the naming rule)
//	  surplus: loose        # off | loose | strict
//
// rules.naming.qualify reads an rule.QualifyMode; rules.boundary reads an
// rule.BoundaryMode; rules.surplus reads an rule.SurplusMode;
// rules.naming.exported is true/false; rules.naming.vocabulary maps a
// namespace to the extra words that satisfy the naming rule for it.
//
// Unknown keys are an error, and the message names the key and the keys the
// section does take.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/scope"
)

// Names are tried in order in each directory.
var Names = []string{".declscope.yaml", ".declscope.yml"}

// BaselineNames are the default baseline file names, discovered the same way
// as the config file when none is configured explicitly.
var BaselineNames = []string{".declscope-baseline.yaml", ".declscope-baseline.yml"}

// The values rules.naming.qualify accepts.
var qualifyModes = rule.QualifyModeSet{rule.QualifyModeAlways, rule.QualifyModeNever, rule.QualifyModeOnDemand}

// The values rules.boundary accepts, from reporting least to most.
var boundaryModes = rule.BoundaryModeSet{rule.BoundaryModeOff, rule.BoundaryModeOn}

// The values rules.surplus accepts, from reporting least to most.
var surplusModes = rule.SurplusModeSet{rule.SurplusModeOff, rule.SurplusModeLoose, rule.SurplusModeStrict}

// boolSetting is a true/false key, with its own error naming the two values it
// takes rather than the parser's "cannot unmarshal".
type boolSetting struct {
	set   bool
	value bool
}

func (b *boolSetting) UnmarshalYAML(node *yaml.Node) error {
	if err := node.Decode(&b.value); err != nil {
		return fmt.Errorf("want true or false")
	}
	b.set = true
	return nil
}

// Each section is a named type so that go-yaml's strict-decoding error can name
// the section a misspelled key sits in, rather than printing the anonymous
// struct's whole type literal.
type defaultsSection struct {
	Unexported string `yaml:"unexported"`
}

// filterSection is the two pattern lists. A file matches a list when it
// matches any pattern in it, and an empty only list places no restriction
// rather than matching nothing.
type filterSection struct {
	Only []string `yaml:"only"`
	Omit []string `yaml:"omit"`
}

type rulesSection struct {
	Naming namingSection `yaml:"naming"`

	// Boundary says whether the boundary rule reports: off or on. Off leaves
	// only the naming rule. A mode rather than a switch, so the key spells
	// the rule's own name.
	Boundary string `yaml:"boundary"`

	// Surplus says how much the surplus rule reports: off, loose or strict.
	// A mode rather than a switch, so the key spells the rule's own name.
	Surplus string `yaml:"surplus"`
}

// namingSection holds the naming rule and its reach. exported decides which
// declarations the rule sees at all; qualify decides when it asks.
type namingSection struct {
	Qualify  string      `yaml:"qualify"`
	Exported boolSetting `yaml:"exported"`

	// Vocabulary maps a namespace to extra words that carry it. The words are
	// spellings, matched exactly as the namespace is, and meant for the
	// irregular few (mouse: wheel, index: indices) — a namespace needing a
	// long list is naming something its file is not about.
	Vocabulary map[string][]string `yaml:"vocabulary"`
}

// File is the on-disk configuration. Every field is optional, and no setting
// has a zero value that means anything, so a field left empty is skipped by
// Apply: omitting a key keeps the built-in default rather than silently
// disabling a rule.
type File struct {
	Defaults defaultsSection `yaml:"defaults"`
	Rules    rulesSection    `yaml:"rules"`

	// Filter narrows and subtracts. only alone keeps nothing outside it, omit
	// alone takes files out of everything, and both together narrow first.
	Filter filterSection `yaml:"filter"`

	// Baseline is a path relative to this config file.
	Baseline string `yaml:"baseline"`

	// path is where this config was read from, used to resolve Baseline.
	path string
}

// Resolve produces the options for analyzing a package directory: built-in
// defaults, with the nearest config file layered on top, and the baseline
// loaded — the one the config names, else the nearest default-named file
// found by FindBaseline. explicit overrides the config lookup. It returns the
// config path that was used, or "" when there was none.
func Resolve(dir, explicit string) (internal.Options, string, error) {
	opts, path, err := resolve(dir, explicit)
	if err != nil {
		return opts, path, err
	}
	if opts.BaselinePath == "" {
		opts.BaselinePath = FindBaseline(dir)
	}
	if opts.BaselinePath != "" {
		set, err := baseline.Load(opts.BaselinePath)
		if err != nil {
			return opts, path, err
		}
		opts.Baseline = set
	}
	return opts, path, nil
}

// ResolveForBaseline produces the options for regenerating the baseline of a
// package directory: the same rules as Resolve, but no baseline is looked up
// or loaded, so the options record the current state from scratch. Loading it
// would let a baseline that fails to parse block its own regeneration, which
// is the one remedy the documentation offers for it.
//
// The third result is the baseline the config file names, resolved against
// the config file, or "" when it names none. Where a package's entries belong
// in that case is the caller's decision — see DefaultBaseline.
func ResolveForBaseline(dir, explicit string) (opts internal.Options, configPath, named string, err error) {
	opts, configPath, err = resolve(dir, explicit)
	named, opts.BaselinePath = opts.BaselinePath, ""
	return opts, configPath, named, err
}

func resolve(dir, explicit string) (internal.Options, string, error) {
	opts := internal.DefaultOptions()

	// An explicit -config names one file and asks for it, so no chain is
	// built: the caller said which rules to use.
	chain := []string{explicit}
	if explicit == "" {
		chain = FindChain(dir)
	}
	path := ""
	if len(chain) > 0 {
		path = chain[len(chain)-1]
	}
	// Outermost first, so a nearer file overrides the keys it states and
	// leaves the rest as the file above set them.
	for _, p := range chain {
		if p == "" {
			continue
		}
		f, err := Load(p)
		if err != nil {
			return opts, p, err
		}
		if err := f.Apply(&opts); err != nil {
			return opts, p, fmt.Errorf("%s: %w", p, err)
		}
	}
	if err := opts.Compile(); err != nil {
		return opts, path, err
	}
	return opts, path, nil
}

// FindChain walks up from dir and returns every config file it passes,
// outermost first. The nearest file is last, so applying the slice in order
// leaves the nearest one's stated keys on top.
//
// The search stops at a module root, exactly as Find does. Config files do
// compose, so a file above the module would reach into it, which is the one
// thing the module boundary is there to prevent.
func FindChain(dir string) []string {
	var found []string
	for dir != "" {
		for _, name := range Names {
			if path := filepath.Join(dir, name); isFile(path) {
				found = append(found, path)
				break
			}
		}
		if isFile(filepath.Join(dir, "go.mod")) {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	slices.Reverse(found)
	return found
}

// Find walks up from dir looking for a config file and returns its path, or
// "" when there is none. The search stops at a module root, so a stray config
// file somewhere above the module cannot silently change its rules.
func Find(dir string) string { return findUp(dir, Names) }

// FindBaseline walks up from dir looking for a default-named baseline file.
// Its presence is what enables suppression, so the name is fixed and the
// search stops at the module root, exactly like the config lookup.
func FindBaseline(dir string) string { return findUp(dir, BaselineNames) }

// DefaultBaseline returns where the entries of a package in dir belong when no
// config file names a baseline: the nearest existing default-named file
// between the package and root, which is what the analyzer will consult, else
// a new file in root, the directory the regeneration was run from.
//
// The search walks the same path as FindBaseline but stops at root. A
// default-named file above root holds entries for packages the run never saw,
// and rewriting it wholesale would drop them; a new file in root shadows it
// for exactly the packages under root instead.
//
// It reports false when root is not on the path at all — the package lies
// outside root, or a module boundary intervenes — because a file written in
// root would never be found from dir, and a baseline whose presence is not
// enough is worse than none. An existing file found on the way is committed
// to only once the walk has confirmed root is below it: for a package outside
// root, the nearest file is above root, and rewriting it has the same cost as
// rewriting one above root for a package inside.
func DefaultBaseline(dir, root string) (string, bool) {
	found := ""
	for dir != "" {
		if found == "" {
			for _, name := range BaselineNames {
				if path := filepath.Join(dir, name); isFile(path) {
					found = path
					break
				}
			}
		}
		if sameDir(dir, root) {
			if found != "" {
				return found, true
			}
			return filepath.Join(root, BaselineNames[0]), true
		}
		if isFile(filepath.Join(dir, "go.mod")) {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
	return "", false
}

func findUp(dir string, names []string) string {
	for dir != "" {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if isFile(path) {
				return path
			}
		}
		if isFile(filepath.Join(dir, "go.mod")) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// sameDir compares two directories by identity rather than spelling, since
// the working directory and the paths go/packages reports can differ through
// a symlink and still be the same place.
func sameDir(a, b string) bool {
	if a == b {
		return true
	}
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(sa, sb)
}

// Load reads and parses a config file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// The path is kept absolute because the exclude patterns and the baseline
	// are resolved against the directory holding it. A -config given relative
	// to the working directory would otherwise name a directory that no
	// reported file path is ever taken from.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	f := File{path: path}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	// Unknown keys are an error rather than a silent no-op: a typo in a rule
	// name would otherwise leave the rule at its default with no sign of it.
	dec.KnownFields(true)
	// An empty document is a valid config that changes nothing.
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, namedKeys(err))
	}
	return &f, nil
}

// The sections a key can sit in, by the type name go-yaml puts in its
// strict-decoding complaint.
var sections = map[string]struct {
	prefix string
	fields any
}{
	"config.File":            {"", File{}},
	"config.defaultsSection": {"defaults.", defaultsSection{}},
	"config.rulesSection":    {"rules.", rulesSection{}},
	"config.namingSection":   {"rules.naming.", namingSection{}},
}

var unknownField = regexp.MustCompile(`^line (\d+): field (\S+) not found in type (\S+)$`)

// namedKeys rewrites go-yaml's strict-decoding complaint, which names the Go
// type it could not fill. For a nested section that type has no name of its
// own, so go-yaml spells the whole struct literal — a line of field tags where
// the author wants the key they misspelled and the ones that would have
// worked.
func namedKeys(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	out := make([]string, 0, len(typeErr.Errors))
	for _, e := range typeErr.Errors {
		m := unknownField.FindStringSubmatch(e)
		if m == nil {
			out = append(out, e)
			continue
		}
		sec, ok := sections[strings.TrimPrefix(m[3], "*")]
		if !ok {
			out = append(out, e)
			continue
		}
		out = append(out, fmt.Sprintf("line %s: unknown key %q (this section takes %s)",
			m[1], sec.prefix+m[2], strings.Join(keysOf(sec.fields), ", ")))
	}
	return errors.New(strings.Join(out, "\n"))
}

// keysOf lists a section's keys as they are spelled in YAML.
func keysOf(section any) []string {
	t := reflect.TypeOf(section)
	names := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		if tag, ok := t.Field(i).Tag.Lookup("yaml"); ok {
			names = append(names, strings.Split(tag, ",")[0])
		}
	}
	return names
}

// Apply layers the file's settings onto opts.
func (f *File) Apply(opts *internal.Options) error {
	if f.Defaults.Unexported != "" {
		s, ok := scope.Parse(f.Defaults.Unexported)
		if !ok {
			return fmt.Errorf("defaults.unexported: unknown scope %q (want package or private)", f.Defaults.Unexported)
		}
		opts.Unexported = s
	}

	if f.Rules.Naming.Qualify != "" {
		m, ok := qualifyModes.Parse(f.Rules.Naming.Qualify)
		if !ok {
			return fmt.Errorf("rules.naming.qualify: unknown mode %q (want %s)", f.Rules.Naming.Qualify, qualifyModes)
		}
		opts.Qualify = m
	}
	if f.Rules.Naming.Exported.set {
		opts.NameExported = f.Rules.Naming.Exported.value
	}
	// The map merges per namespace rather than replacing: adding one word in a
	// nested config should not cost restating every other.
	if len(f.Rules.Naming.Vocabulary) > 0 {
		merged := make(map[string][]string, len(opts.Vocabulary)+len(f.Rules.Naming.Vocabulary))
		for ns, words := range opts.Vocabulary {
			merged[ns] = words
		}
		for ns, words := range f.Rules.Naming.Vocabulary {
			merged[ns] = words
		}
		opts.Vocabulary = merged
	}
	if f.Rules.Boundary != "" {
		m, ok := boundaryModes.Parse(f.Rules.Boundary)
		if !ok {
			return fmt.Errorf("rules.boundary: unknown mode %q (want %s)", f.Rules.Boundary, boundaryModes)
		}
		opts.Boundary = m
	}
	if f.Rules.Surplus != "" {
		m, ok := surplusModes.Parse(f.Rules.Surplus)
		if !ok {
			return fmt.Errorf("rules.surplus: unknown mode %q (want %s)", f.Rules.Surplus, surplusModes)
		}
		opts.Surplus = m
	}
	// only intersects and omit unions, so each stating file adds to what is
	// already there rather than replacing it. A config file can narrow what is
	// read and never widen it.
	base := f.FilterBase()
	if len(f.Filter.Only) > 0 {
		opts.Only = append(opts.Only, patternsIn(f.Filter.Only, base))
	}
	if len(f.Filter.Omit) > 0 {
		opts.Omit = append(opts.Omit, patternsIn(f.Filter.Omit, base)...)
	}
	if f.Baseline != "" {
		opts.BaselinePath = f.BaselinePath()
	}
	return nil
}

// patternsIn pairs each pattern with the directory it was written in, which
// is what lets one chain hold patterns anchored at several depths.
func patternsIn(patterns []string, base string) []internal.FilterPattern {
	out := make([]internal.FilterPattern, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, internal.FilterPattern{Pattern: p, Base: base})
	}
	return out
}

// FilterBase is the directory the filter patterns are relative to: the one
// holding this config file, the way BaselinePath resolves the baseline. A
// path written in a file means a path from that file, and every other path
// key here already works that way.
//
// Resolving here rather than at match time is what lets the pattern itself be
// anchored: ** regains its meaning, since a pattern no longer matches at every
// depth by construction.
//
// Empty when the file did not come from disk.
func (f *File) FilterBase() string {
	if f.path == "" {
		return ""
	}
	return filepath.Dir(f.path)
}

// BaselinePath resolves the configured baseline against the config file's own
// directory, so that a config can be moved without rewriting the path.
func (f *File) BaselinePath() string {
	if f.Baseline == "" {
		return ""
	}
	if filepath.IsAbs(f.Baseline) || f.path == "" {
		return f.Baseline
	}
	return filepath.Join(filepath.Dir(f.path), f.Baseline)
}
