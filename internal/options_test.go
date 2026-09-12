package internal

import "testing"

func TestExcluded(t *testing.T) {
	opts := DefaultOptions()
	opts.Exclude = []string{"**/mock_*.go", "vendor/**", "gen.go"}
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		want bool
	}{
		{"/repo/pkg/mock_user.go", true},
		{"mock_user.go", true},
		{"/repo/pkg/user.go", false},
		{"/repo/pkg/mockuser.go", false},
		{"vendor/foo/bar.go", true},
		{"/repo/vendor/foo/bar.go", true},
		{"/repo/gen.go", true},
		{"/repo/pkg/gen.go", true},
		{"/repo/pkg/regen.go", false},
		// * must not reach across separators.
		{"/repo/a/b/vendor.go", false},
	}
	for _, tt := range tests {
		if got := opts.Excluded(tt.path); got != tt.want {
			t.Errorf("Excluded(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestExcludedEscapesRegexpMetacharacters checks that a pattern is read as a
// glob and never as a regexp, so a bracket in a path is matched literally
// rather than compiling into a character class.
func TestExcludedEscapesRegexpMetacharacters(t *testing.T) {
	opts := DefaultOptions()
	opts.Exclude = []string{"gen[1].go"}
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	if !opts.Excluded("/repo/gen[1].go") {
		t.Error("a bracket should match literally")
	}
	if opts.Excluded("/repo/gen1.go") {
		t.Error("a bracket should not compile into a character class")
	}
}
