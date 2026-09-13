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

// TestModeApplies pins the one predicate both naming rules gate on: ModeAlways
// for every package, ModeNever for none, ModeOnDemand only once a package has a
// second namespace to distinguish.
func TestModeApplies(t *testing.T) {
	tests := []struct {
		mode       Mode
		namespaces int
		want       bool
	}{
		{ModeAlways, 0, true},
		{ModeAlways, 1, true},
		{ModeAlways, 2, true},
		{ModeNever, 0, false},
		{ModeNever, 1, false},
		{ModeNever, 2, false},
		{ModeOnDemand, 0, false},
		{ModeOnDemand, 1, false},
		{ModeOnDemand, 2, true},
		{ModeOnDemand, 3, true},
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
	all := ModeSet{ModeAlways, ModeNever, ModeOnDemand}
	for m, want := range map[Mode]string{
		ModeAlways:   "always",
		ModeNever:    "never",
		ModeOnDemand: "ondemand",
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
	all := ModeSet{ModeAlways, ModeNever, ModeOnDemand}
	binary := ModeSet{ModeAlways, ModeNever}
	tests := []struct {
		set   ModeSet
		value string
		want  Mode
		ok    bool
	}{
		{all, "always", ModeAlways, true},
		{all, "never", ModeNever, true},
		{all, "ondemand", ModeOnDemand, true},
		{all, "true", 0, false},
		{all, "false", 0, false},
		{all, "sometimes", 0, false},
		{all, "", 0, false},
		{binary, "always", ModeAlways, true},
		{binary, "never", ModeNever, true},
		{binary, "ondemand", 0, false},
		{ModeSet{ModeNever}, "always", 0, false},
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
		{ModeSet{ModeAlways, ModeNever, ModeOnDemand}, "always, never or ondemand"},
		{ModeSet{ModeAlways, ModeNever}, "always or never"},
		{ModeSet{ModeNever}, "never"},
		{ModeSet{}, ""},
	}
	for _, tt := range tests {
		if got := tt.set.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}
