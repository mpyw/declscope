package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// filterPatternsAt pairs patterns with the directory a config file would have
// written them in, which is what the chain carries.
func filterPatternsAt(base string, patterns ...string) []FilterPattern {
	out := make([]FilterPattern, 0, len(patterns))
	for _, pattern := range patterns {
		out = append(out, FilterPattern{Pattern: pattern, Base: base})
	}
	return out
}

// filterOnlyAt is the single group one config file's only list produces.
func filterOnlyAt(base string, patterns ...string) [][]FilterPattern {
	return [][]FilterPattern{filterPatternsAt(base, patterns...)}
}

// TestFilterAnchoring pins which patterns speak about one place and which
// speak about any depth. The split follows .gitignore: a pattern holding a
// separator is anchored to the directory of the config file that states it,
// unless the separator is part of a leading **.
func TestFilterAnchoring(t *testing.T) {
	opts := DefaultOptions()
	opts.Omit = filterPatternsAt("/repo",
		"**/mock_*.go", // any depth, anywhere
		"gen.go",       // any depth: a bare name carries no place
		"vendor/**",    // the one beside the config
		"/tools/**",    // the same, spelled the way .gitignore anchors
		"a/b/*.go",     // one named place, one level deep
		"./build/**",   // one of the ways a person writes "beside this file"
	)
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		want bool
	}{
		// **/ floats, including to zero depth and outside the base.
		{"/repo/pkg/mock_user.go", true},
		{"/repo/mock_user.go", true},
		{"/elsewhere/mock_user.go", true},
		{"/repo/pkg/mockuser.go", false},

		// A bare name floats too, which is what it means in a .gitignore.
		{"/repo/gen.go", true},
		{"/repo/pkg/deep/gen.go", true},
		{"/repo/pkg/regen.go", false},

		// A pattern with a separator names one place and no other.
		{"/repo/vendor/foo/bar.go", true},
		{"/repo/pkg/vendor/foo.go", false},
		{"/elsewhere/vendor/foo/bar.go", false},

		// A leading separator anchors rather than naming the filesystem root.
		{"/repo/tools/x.go", true},
		{"/repo/pkg/tools/x.go", false},

		// * does not reach across separators.
		{"/repo/a/b/c.go", true},
		{"/repo/a/b/c/d.go", false},
		{"/repo/a/b.go", false},

		// A leading ./ names the directory beside the config file, not a
		// directory called ".", so it anchors exactly as "build/**" does.
		{"/repo/build/x.go", true},
		{"/repo/pkg/build/x.go", false},
	}
	for _, tt := range tests {
		if got := opts.Skips(tt.path); got != tt.want {
			t.Errorf("Skips(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestFilterWithoutBase checks the fallback for options that no config file
// produced: with no directory to call "here", an anchored pattern would match
// nothing at all, so every pattern floats instead.
func TestFilterWithoutBase(t *testing.T) {
	opts := DefaultOptions()
	opts.Omit = filterPatternsAt("", "vendor/**")
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/repo/vendor/foo.go", "/elsewhere/deep/vendor/foo.go"} {
		if !opts.Skips(path) {
			t.Errorf("Skips(%q) = false, want true", path)
		}
	}
	if opts.Skips("/repo/pkg/foo.go") {
		t.Error("an unrelated path should not be skipped")
	}
}

// TestFilterThroughSymlink checks that anchoring survives the two spellings a
// directory can have. Resolving the pattern against the config path as text
// would match nothing whenever the driver reported paths through the other
// spelling, and would do so without an error to say why.
func TestFilterThroughSymlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(real, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The file has to exist: resolving the spelling of a path is what makes
	// the two agree, and a path that names nothing cannot be resolved.
	if err := os.WriteFile(filepath.Join(real, "vendor", "foo.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, base := range []string{real, link} {
		opts := DefaultOptions()
		opts.Omit = filterPatternsAt(base, "vendor/**")
		if err := opts.Compile(); err != nil {
			t.Fatal(err)
		}
		for _, spelling := range []string{real, link} {
			path := filepath.Join(spelling, "vendor", "foo.go")
			if !opts.Skips(path) {
				t.Errorf("base %q did not skip %q", base, path)
			}
		}
	}
}

// TestFilterEscapesRegexpMetacharacters checks that a pattern is read as a
// glob and never as a regexp, so a bracket in a path is matched literally
// rather than compiling into a character class.
func TestFilterEscapesRegexpMetacharacters(t *testing.T) {
	opts := DefaultOptions()
	opts.Omit = filterPatternsAt("/repo", "sub/gen[1].go")
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	if !opts.Skips("/repo/sub/gen[1].go") {
		t.Error("a bracket should match literally")
	}
	if opts.Skips("/repo/sub/gen1.go") {
		t.Error("a bracket should not compile into a character class")
	}
}

// TestFilterOnlyNarrows checks the half exclude could not express. An empty
// only list places no restriction, and a non-empty one keeps nothing outside
// it, which is not what "matches nothing" would do.
func TestFilterOnlyNarrows(t *testing.T) {
	opts := DefaultOptions()
	if opts.Skips("/repo/anywhere.go") {
		t.Error("no only list should place no restriction")
	}

	opts.Only = filterOnlyAt("/repo", "internal/**")
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		"/repo/internal/a.go":      false,
		"/repo/internal/sub/b.go":  false,
		"/repo/cmd/c.go":           true,
		"/elsewhere/internal/d.go": true,
	} {
		if got := opts.Skips(path); got != want {
			t.Errorf("Skips(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestFilterMatchesAnyPattern checks that each list is a disjunction. One
// pattern matching is enough, in either list.
func TestFilterMatchesAnyPattern(t *testing.T) {
	opts := DefaultOptions()
	opts.Only = filterOnlyAt("/repo", "internal/**", "cmd/**")
	opts.Omit = filterPatternsAt("/repo", "**/mock_*.go", "**/*_gen.go")
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		// Either only pattern admits.
		"/repo/internal/a.go": false,
		"/repo/cmd/b.go":      false,
		// Neither does.
		"/repo/tools/c.go": true,
		// Either omit pattern takes it back out, from inside only.
		"/repo/internal/mock_a.go": true,
		"/repo/cmd/b_gen.go":       true,
	} {
		if got := opts.Skips(path); got != want {
			t.Errorf("Skips(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestFilterOmitBitesInsideOnly is the interaction between the two lists,
// stated on its own because it is the one a reader asks about: a file the only
// list admits is still taken out when omit names it.
//
// The order the two are applied in is not observable. Both ask about one path,
// so narrowing before subtracting and subtracting before narrowing name the
// same set. This pins the set, not an order.
func TestFilterOmitBitesInsideOnly(t *testing.T) {
	opts := DefaultOptions()
	opts.Only = filterOnlyAt("/repo", "internal/**")
	opts.Omit = filterPatternsAt("/repo", "internal/legacy/**")
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		"/repo/internal/a.go":        false,
		"/repo/internal/legacy/b.go": true,
		"/repo/cmd/c.go":             true,
		"/repo/legacy/d.go":          true, // outside only, omit irrelevant
	} {
		if got := opts.Skips(path); got != want {
			t.Errorf("Skips(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestFilterQuestionMarkMatchesOneCharacter checks the third glob
// metacharacter. ? stops at a separator for the same reason * does: a pattern
// naming one path segment must not silently reach into another.
func TestFilterQuestionMarkMatchesOneCharacter(t *testing.T) {
	opts := DefaultOptions()
	opts.Omit = filterPatternsAt("/repo", "sub/gen?.go", "a?b/**")
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		"/repo/sub/gen1.go":  true,
		"/repo/sub/gen10.go": false, // one character, not any number
		"/repo/sub/gen.go":   false, // and not zero
		"/repo/axb/c.go":     true,
		"/repo/a/b/c.go":     false, // ? does not reach across a separator
	} {
		if got := opts.Skips(path); got != want {
			t.Errorf("Skips(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestFilterRefusesPatternLeavingItsDirectory checks the one pattern that is
// rejected rather than compiled. A path is matched relative to the directory
// of the config file that states the pattern and never holds a "..", so such a
// pattern could only ever match nothing — and a filter that silently matches
// nothing is the failure a reader has no way to see.
func TestFilterRefusesPatternLeavingItsDirectory(t *testing.T) {
	for _, pattern := range []string{"../sibling/**", "sub/../../up.go"} {
		opts := DefaultOptions()
		opts.Omit = filterPatternsAt("/repo", pattern)
		err := opts.Compile()
		if err == nil {
			t.Errorf("Compile() accepted %q, want a refusal", pattern)
			continue
		}
		if !strings.Contains(err.Error(), pattern) {
			t.Errorf("the refusal should quote the pattern: %v", err)
		}
	}
}
