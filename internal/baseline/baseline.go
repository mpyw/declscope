// Package baseline records violations that already exist in a codebase so that
// adopting declscope does not require fixing them all at once.
//
// A baseline is regenerated wholesale rather than edited:
//
//	declscope baseline ./...
//
// Entries are therefore never stale — an entry for a violation that has since
// been fixed simply disappears on the next regeneration, and the diff shows
// what was fixed. This is why the analyzer does not report unmatched entries:
// a package's test variant sees references that the non-test variant does not,
// so "this entry matched nothing" is not a reliable signal from inside a
// single pass.
//
// A baseline suppresses; it does not endorse. Nothing is written into the
// source, so the naming convention still applies to every new declaration and
// an entry can only be removed by actually fixing the violation.
package baseline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/mpyw/declscope/internal/rule"
)

// Key identifies a violation independently of its position.
//
// Position is deliberately not part of the identity: a package-level
// declaration is unique within its package by name, and a member is unique by
// owner and name, so a key survives the code moving within its file and the
// package being reformatted.
//
// The namespace is part of it, and is the one thing that does not survive a
// move between files. It has to be: boundary is a statement about which
// namespaces a use crosses, so the same declaration in a different file is a
// different violation, and a key blind to that would go on suppressing a
// crossing nobody recorded.
type Key struct {
	// Package is the import path of the package declaring the violation.
	Package string
	// Rule is the check that fired.
	Rule rule.Rule
	// Namespace bounds the declaration: the namespace of its file, or of its
	// type's file for a field. "(core)" for the core namespace, which has no
	// name of its own.
	Namespace string
	// Decl names the declaration: "helper" for a package-level one,
	// "User.name" for a member.
	Decl string
}

// Set is a loaded baseline.
type Set struct {
	keys map[Key]bool
}

// Has reports whether the violation is already recorded.
func (s *Set) Has(k Key) bool {
	if s == nil {
		return false
	}
	return s.keys[k]
}

// Len returns the number of recorded violations.
func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	return len(s.keys)
}

// file is the on-disk shape: package path -> rule -> namespace -> declarations.
//
// Nesting rather than spelling the namespace into each entry keeps the file
// readable as a list of what each unit still owes, and keeps a move legible:
// the declaration leaves one block and appears in another, where a flat
// "user/helper" would leave two unrelated lines in the diff.
type file struct {
	Packages map[string]map[string]map[string][]string `yaml:"packages"`
}

const header = `# declscope baseline
#
# Violations recorded here are suppressed; new ones are still reported.
# Regenerate with:
#
#     declscope baseline ./...
#
# Do not edit by hand. An entry disappears when its violation is fixed, so the
# diff of a regeneration is the record of what was cleaned up.
`

// Load reads a baseline file. A missing file is not an error: it yields an
// empty set, so a configured but not yet generated baseline behaves as none.
func Load(path string) (*Set, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Set{keys: map[Key]bool{}}, nil
		}
		return nil, err
	}
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && err != io.EOF {
		// A baseline is generated, never written by hand, so anything that
		// fails to parse is answered the same way: generate it again.
		return nil, fmt.Errorf("%s: %w (regenerate it with `declscope baseline ./...`)", path, err)
	}
	s := &Set{keys: map[Key]bool{}}
	for pkg, rules := range f.Packages {
		for name, spaces := range rules {
			for ns, decls := range spaces {
				for _, decl := range decls {
					s.keys[Key{Package: pkg, Rule: rule.Rule(name), Namespace: ns, Decl: decl}] = true
				}
			}
		}
	}
	return s, nil
}

// Save writes keys to path, sorted so that regenerating an unchanged codebase
// produces no diff. It returns the number of entries written, which is the
// count to report: a package and its test variant hand over the same key
// twice, and only one of them lands in the file.
func Save(path string, keys []Key) (int, error) {
	f := file{Packages: map[string]map[string]map[string][]string{}}
	n := 0
	for _, k := range keys {
		rules, ok := f.Packages[k.Package]
		if !ok {
			rules = map[string]map[string][]string{}
			f.Packages[k.Package] = rules
		}
		name := string(k.Rule)
		spaces, ok := rules[name]
		if !ok {
			spaces = map[string][]string{}
			rules[name] = spaces
		}
		if !slices.Contains(spaces[k.Namespace], k.Decl) {
			spaces[k.Namespace] = append(spaces[k.Namespace], k.Decl)
			n++
		}
	}
	for _, rules := range f.Packages {
		for _, spaces := range rules {
			for ns := range spaces {
				slices.Sort(spaces[ns])
			}
		}
	}

	var buf bytes.Buffer
	buf.WriteString(header)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return 0, err
	}
	if err := enc.Close(); err != nil {
		return 0, err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}
	return n, nil
}
