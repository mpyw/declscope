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

// TestModeApplies pins the one predicate both naming rules gate on: Always
// for every package, Never for none, OnDemand only once a package has a
// second namespace to distinguish.
func TestModeApplies(t *testing.T) {
	tests := []struct {
		mode       Mode
		namespaces int
		want       bool
	}{
		{Always, 0, true},
		{Always, 1, true},
		{Always, 2, true},
		{Never, 0, false},
		{Never, 1, false},
		{Never, 2, false},
		{OnDemand, 0, false},
		{OnDemand, 1, false},
		{OnDemand, 2, true},
		{OnDemand, 3, true},
	}
	for _, tt := range tests {
		if got := tt.mode.Applies(tt.namespaces); got != tt.want {
			t.Errorf("%v.Applies(%d) = %v, want %v", tt.mode, tt.namespaces, got, tt.want)
		}
	}
}

// TestModeString pins the spelling a diagnostic or an error would use to the
// one the settings take, and that it parses back.
func TestModeString(t *testing.T) {
	all := ModeSet{Always, Never, OnDemand}
	for m, want := range map[Mode]string{
		Always:   "always",
		Never:    "never",
		OnDemand: "ondemand",
	} {
		if got := m.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
		if back, ok := all.Parse(m.String()); !ok || back != m {
			t.Errorf("Parse(%q) = %v, %v; want %v", m.String(), back, ok, m)
		}
	}
}

// TestModeSetParse checks that a set accepts its members and nothing else: a
// mode outside the set is refused the same way as a word that is no mode.
func TestModeSetParse(t *testing.T) {
	all := ModeSet{Always, Never, OnDemand}
	binary := ModeSet{Always, Never}
	tests := []struct {
		set   ModeSet
		value string
		want  Mode
		ok    bool
	}{
		{all, "always", Always, true},
		{all, "never", Never, true},
		{all, "ondemand", OnDemand, true},
		{all, "true", 0, false},
		{all, "false", 0, false},
		{all, "sometimes", 0, false},
		{all, "", 0, false},
		{binary, "always", Always, true},
		{binary, "never", Never, true},
		{binary, "ondemand", 0, false},
		{ModeSet{Never}, "always", 0, false},
	}
	for _, tt := range tests {
		got, ok := tt.set.Parse(tt.value)
		if ok != tt.ok || got != tt.want {
			t.Errorf("%v.Parse(%v) = %v, %v; want %v, %v", tt.set, tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

// TestModeSetString pins how a set names its members in an error message.
func TestModeSetString(t *testing.T) {
	tests := []struct {
		set  ModeSet
		want string
	}{
		{ModeSet{Always, Never, OnDemand}, "always, never or ondemand"},
		{ModeSet{Always, Never}, "always or never"},
		{ModeSet{Never}, "never"},
		{ModeSet{}, ""},
	}
	for _, tt := range tests {
		if got := tt.set.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}
