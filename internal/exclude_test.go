package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExcludeAnchoring pins which patterns speak about one place and which
// speak about any depth. The split follows .gitignore: a pattern holding a
// separator is anchored to the directory of the config file that states it,
// unless the separator is part of a leading **.
func TestExcludeAnchoring(t *testing.T) {
	opts := DefaultOptions()
	opts.ExcludeBase = "/repo"
	opts.Exclude = []string{
		"**/mock_*.go", // any depth, anywhere
		"gen.go",       // any depth: a bare name carries no place
		"vendor/**",    // the one beside the config
		"/tools/**",    // the same, spelled the way .gitignore anchors
		"a/b/*.go",     // one named place, one level deep
	}
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
	}
	for _, tt := range tests {
		if got := opts.Excluded(tt.path); got != tt.want {
			t.Errorf("Excluded(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestExcludeWithoutBase checks the fallback for options that no config file
// produced: with no directory to call "here", an anchored pattern would match
// nothing at all, so every pattern floats instead.
func TestExcludeWithoutBase(t *testing.T) {
	opts := DefaultOptions()
	opts.Exclude = []string{"vendor/**"}
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/repo/vendor/foo.go", "/elsewhere/deep/vendor/foo.go"} {
		if !opts.Excluded(path) {
			t.Errorf("Excluded(%q) = false, want true", path)
		}
	}
	if opts.Excluded("/repo/pkg/foo.go") {
		t.Error("an unrelated path should not be excluded")
	}
}

// TestExcludeThroughSymlink checks that anchoring survives the two spellings a
// directory can have. Resolving the pattern against the config path as text
// would match nothing whenever the driver reported paths through the other
// spelling, and would do so without an error to say why.
func TestExcludeThroughSymlink(t *testing.T) {
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
		opts.ExcludeBase = base
		opts.Exclude = []string{"vendor/**"}
		if err := opts.Compile(); err != nil {
			t.Fatal(err)
		}
		for _, spelling := range []string{real, link} {
			path := filepath.Join(spelling, "vendor", "foo.go")
			if !opts.Excluded(path) {
				t.Errorf("base %q did not exclude %q", base, path)
			}
		}
	}
}

// TestExcludeEscapesRegexpMetacharacters checks that a pattern is read as a
// glob and never as a regexp, so a bracket in a path is matched literally
// rather than compiling into a character class.
func TestExcludeEscapesRegexpMetacharacters(t *testing.T) {
	opts := DefaultOptions()
	opts.ExcludeBase = "/repo"
	opts.Exclude = []string{"sub/gen[1].go"}
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	if !opts.Excluded("/repo/sub/gen[1].go") {
		t.Error("a bracket should match literally")
	}
	if opts.Excluded("/repo/sub/gen1.go") {
		t.Error("a bracket should not compile into a character class")
	}
}
