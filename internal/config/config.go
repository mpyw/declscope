// Package config loads declscope's YAML configuration.
//
// A config file is optional. When present it is named .declscope.yaml (or
// .yml) and is looked up from the directory of the package being analyzed
// upwards, so that a subtree can relax the rules without the rest of the
// module following suit.
//
//	defaults:
//	  exported: public     # public | package | file
//	  unexported: file
//
//	rules:
//	  promote: ondemand    # true | false | ondemand (only once a package has two namespaces)
//	  demote: false        # and where it is not required, forbid it
//	  members: true        # bound unexported methods/fields by their type's namespace
//
//	exclude:
//	  - "**/mock_*.go"
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/scope"
)

// Names are tried in order in each directory.
var Names = []string{".declscope.yaml", ".declscope.yml"}

// BaselineNames are the default baseline file names, discovered the same way
// as the config file when none is configured explicitly.
var BaselineNames = []string{".declscope-baseline.yaml", ".declscope-baseline.yml"}

// File is the on-disk configuration. Every field is optional; pointers and
// empty strings distinguish "not set" from "set to the zero value", so that
// omitting a key keeps the built-in default rather than silently disabling a
// rule.
type File struct {
	Defaults struct {
		Exported   string `yaml:"exported"`
		Unexported string `yaml:"unexported"`
	} `yaml:"defaults"`

	Rules struct {
		// Promote is tri-state, so it arrives as a bool or as a string.
		Promote any   `yaml:"promote"`
		Demote  *bool `yaml:"demote"`
		Members *bool `yaml:"members"`
	} `yaml:"rules"`

	Exclude []string `yaml:"exclude"`

	// Baseline is a path relative to this config file.
	Baseline string `yaml:"baseline"`

	// path is where this config was read from, used to resolve Baseline.
	path string
}

// Resolve produces the options for a package directory: built-in defaults,
// with the nearest config file layered on top. explicit overrides the lookup.
// It returns the config path that was used, or "" when there was none.
func Resolve(dir, explicit string) (internal.Options, string, error) {
	opts := internal.DefaultOptions()

	path := explicit
	if path == "" {
		path = Find(dir)
	}
	if path != "" {
		f, err := Load(path)
		if err != nil {
			return opts, path, err
		}
		if err := f.Apply(&opts); err != nil {
			return opts, path, fmt.Errorf("%s: %w", path, err)
		}
	}
	if opts.BaselinePath == "" {
		opts.BaselinePath = FindBaseline(dir)
	}
	if err := opts.Compile(); err != nil {
		return opts, path, err
	}
	return opts, path, nil
}

// Find walks up from dir looking for a config file and returns its path, or
// "" when there is none. The search stops at a module root, so a stray config
// file somewhere above the module cannot silently change its rules.
func Find(dir string) string { return findUp(dir, Names) }

// FindBaseline walks up from dir looking for a default-named baseline file.
// Its presence is what enables suppression, so the name is fixed and the
// search stops at the module root, exactly like the config lookup.
func FindBaseline(dir string) string { return findUp(dir, BaselineNames) }

func findUp(dir string, names []string) string {
	for dir != "" {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if st, err := os.Stat(path); err == nil && !st.IsDir() {
				return path
			}
		}
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
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

// Load reads and parses a config file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := File{path: path}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	// Unknown keys are an error rather than a silent no-op: a typo in a rule
	// name would otherwise leave the rule at its default with no sign of it.
	dec.KnownFields(true)
	// An empty document is a valid config that changes nothing.
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &f, nil
}

// Apply layers the file's settings onto opts.
func (f *File) Apply(opts *internal.Options) error {
	for _, field := range []struct {
		name  string
		value string
		dst   *scope.Scope
	}{
		{"defaults.exported", f.Defaults.Exported, &opts.Exported},
		{"defaults.unexported", f.Defaults.Unexported, &opts.Unexported},
	} {
		if field.value == "" {
			continue
		}
		s, ok := scope.Parse(field.value)
		if !ok {
			return fmt.Errorf("%s: unknown scope %q (want public, package or file)", field.name, field.value)
		}
		*field.dst = s
	}

	if f.Rules.Promote != nil {
		mode, ok := internal.ParsePromoteMode(f.Rules.Promote)
		if !ok {
			return fmt.Errorf("rules.promote: want true, false or ondemand, got %v", f.Rules.Promote)
		}
		opts.Promote = mode
	}
	if f.Rules.Demote != nil {
		opts.CheckDemote = *f.Rules.Demote
	}
	if f.Rules.Members != nil {
		opts.CheckMembers = *f.Rules.Members
	}
	if f.Exclude != nil {
		opts.Exclude = f.Exclude
	}
	if f.Baseline != "" {
		opts.BaselinePath = f.BaselinePath()
	}
	return nil
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
