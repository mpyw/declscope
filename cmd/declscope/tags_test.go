package main_test

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tagsModule is a module whose one package exists only under the special
// tag, so that a command which does not apply the tag fails on it.
func tagsModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, "go.mod", testModule)
	writeTree(t, root, "tagged/t.go", "//go:build special\n\npackage tagged\n\nfunc T() {}\n")
	writeTree(t, root, "empty.yaml", "rules: {}\n")
	return root
}

// TestTagsAnalyzer pins that the analyzer applies -tags in every spelling
// package flag accepts, although the driver's own -tags does nothing.
func TestTagsAnalyzer(t *testing.T) {
	root := tagsModule(t)
	if out, code := runIn(t, bin, root, "./tagged"); code != 1 || !strings.Contains(out, "build constraints exclude all Go files") {
		t.Fatalf("exit %d, want the tagged package excluded without -tags:\n%s", code, out)
	}
	for _, args := range [][]string{
		{"-tags=special"},
		{"-tags", "special"},
		{"--tags=special"},
		{"--tags", "special"},
		{"-tags=other,special"},
		// The last -tags wins, as for the go command.
		{"-tags=other", "-tags=special"},
	} {
		out, code := runIn(t, bin, root, append(args, "./tagged")...)
		if code != 0 {
			t.Errorf("%v: exit %d, want the tag applied:\n%s", args, code, out)
		}
	}
	// A -tags with no value is left for the driver, which reports it.
	if out, code := runIn(t, bin, root, "-tags"); code != 2 || !strings.Contains(out, "flag needs an argument: -tags") {
		t.Errorf("exit %d, want the driver's report of the missing value:\n%s", code, out)
	}
	// -tags= clears the tags, and a flag after the first package is a package.
	for _, args := range [][]string{{"-tags=special", "-tags=", "./tagged"}, {"./tagged", "-tags=special"}} {
		if out, code := runIn(t, bin, root, args...); code == 0 {
			t.Errorf("%v: exit 0, want the tag not applied:\n%s", args, out)
		}
	}
}

// TestTagsOverGOFLAGS pins that -tags overrides the tags GOFLAGS holds, as it
// does for the go command.
func TestTagsOverGOFLAGS(t *testing.T) {
	root := tagsModule(t)
	t.Setenv("GOFLAGS", "-tags=special")
	if out, code := runIn(t, bin, root, "./tagged"); code != 0 {
		t.Fatalf("exit %d, want GOFLAGS applied:\n%s", code, out)
	}
	if out, code := runIn(t, bin, root, "-tags=other", "./tagged"); code == 0 {
		t.Fatalf("exit 0, want -tags=other to replace GOFLAGS' tags:\n%s", out)
	}
}

// tagsFlagLine is one flag in the driver's help: its name, and the name of
// its value when it takes one.
var tagsFlagLine = regexp.MustCompile(`(?m)^  -(\S+)(?: (\S+))?`)

// TestTagsAfterEveryFlag pins that -tags is found after each of the driver's
// flags. A flag that takes a value must have that value skipped, or the value
// reads as the first package and the -tags after it as another. A flag that
// takes none must not swallow the -tags. The flags come from the help, so a
// flag x/tools adds is checked too: one taking a value fails here until
// tagsValueFlags and the values below know of it.
func TestTagsAfterEveryFlag(t *testing.T) {
	root := tagsModule(t)
	help, _ := runIn(t, bin, root, "-help")
	values := map[string]string{
		"c":          "0",
		"config":     filepath.Join(root, "empty.yaml"),
		"cpuprofile": filepath.Join(t.TempDir(), "cpu"),
		"debug":      "",
		"memprofile": filepath.Join(t.TempDir(), "mem"),
		"trace":      filepath.Join(t.TempDir(), "trace"),
	}
	// Both print something and exit before any package is loaded.
	skipped := map[string]bool{"V": true, "flags": true, "tags": true}
	seen := 0
	for _, m := range tagsFlagLine.FindAllStringSubmatch(help, -1) {
		name, takesValue := m[1], m[2] != ""
		if skipped[name] {
			continue
		}
		seen++
		args := []string{"-" + name}
		if takesValue {
			value, ok := values[name]
			if !ok {
				t.Errorf("-%s takes a value: add it to tagsValueFlags, and a harmless value here", name)
				continue
			}
			args = append(args, value)
		}
		args = append(args, "-tags", "special", "./tagged")
		if out, code := runIn(t, bin, root, args...); code != 0 {
			t.Errorf("%v: exit %d, want the tag applied:\n%s", args, code, out)
		}
	}
	if seen < 10 {
		t.Fatalf("read only %d flags from the help, want the driver's whole list:\n%s", seen, help)
	}
}

// TestTagsSubcommands pins that every subcommand that loads packages takes
// -tags, as go vet does.
func TestTagsSubcommands(t *testing.T) {
	root := tagsModule(t)
	for _, args := range [][]string{
		{"baseline", "-o", filepath.Join(t.TempDir(), "b.yaml")},
		{"survey"},
		{"inspect"},
	} {
		if out, code := runIn(t, bin, root, append(args, "./tagged")...); code == 0 {
			t.Errorf("%v: exit 0 without -tags, want the tagged package excluded:\n%s", args, out)
		}
		if out, code := runIn(t, bin, root, append(args, "-tags=special", "./tagged")...); code != 0 {
			t.Errorf("%v: exit %d with -tags, want the tag applied:\n%s", args, code, out)
		}
	}
}

// TestTagsHelp pins that every command documents -tags, the analyzer's
// included: the driver's own help says it has no effect.
func TestTagsHelp(t *testing.T) {
	const want = "  -tags list\n    \tcomma-separated list of build tags to consider satisfied, as for go build\n"
	for _, args := range [][]string{{"-help"}, {"baseline", "-h"}, {"survey", "-h"}, {"inspect", "-h"}, {"shrink", "-h"}} {
		out, _ := runIn(t, bin, t.TempDir(), args...)
		if !strings.Contains(out, want) {
			t.Errorf("%v: missing %q in:\n%s", args, want, out)
		}
	}
}
