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
//	  prefixed: package
//
//	rules:
//	  members: true          # bound unexported methods/fields by their type's namespace
//	  foreign-methods: false # report unexported methods grown on another namespace's type
//	  demotion: false        # report namespace prefixes that claim more reach than they use
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

// File is the on-disk configuration. Every field is optional; pointers and
// empty strings distinguish "not set" from "set to the zero value", so that
// omitting a key keeps the built-in default rather than silently disabling a
// rule.
type File struct {
	Defaults struct {
		Exported   string `yaml:"exported"`
		Unexported string `yaml:"unexported"`
		Prefixed   string `yaml:"prefixed"`
	} `yaml:"defaults"`

	Rules struct {
		Members        *bool `yaml:"members"`
		ForeignMethods *bool `yaml:"foreign-methods"`
		Demotion       *bool `yaml:"demotion"`
	} `yaml:"rules"`

	Exclude []string `yaml:"exclude"`
}

// Find walks up from dir looking for a config file and returns its path, or
// "" when there is none. The search stops at a module root, so a stray config
// file somewhere above the module cannot silently change its rules.
func Find(dir string) string {
	for dir != "" {
		for _, name := range Names {
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
	var f File
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
		{"defaults.prefixed", f.Defaults.Prefixed, &opts.Prefixed},
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

	if f.Rules.Members != nil {
		opts.CheckMembers = *f.Rules.Members
	}
	if f.Rules.ForeignMethods != nil {
		opts.CheckForeignMethods = *f.Rules.ForeignMethods
	}
	if f.Rules.Demotion != nil {
		opts.CheckDemotion = *f.Rules.Demotion
	}
	if f.Exclude != nil {
		opts.Exclude = f.Exclude
	}
	return nil
}
