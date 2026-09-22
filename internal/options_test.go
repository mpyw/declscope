package internal

import "testing"

// TestQualifyModeApplies pins the one predicate both naming rules gate on: QualifyModeAlways
// for every package, QualifyModeNever for none, QualifyModeOnDemand only once a package has a
// second namespace to distinguish.
func TestQualifyModeApplies(t *testing.T) {
	tests := []struct {
		mode       QualifyMode
		namespaces int
		want       bool
	}{
		{QualifyModeAlways, 0, true},
		{QualifyModeAlways, 1, true},
		{QualifyModeAlways, 2, true},
		{QualifyModeNever, 0, false},
		{QualifyModeNever, 1, false},
		{QualifyModeNever, 2, false},
		{QualifyModeOnDemand, 0, false},
		{QualifyModeOnDemand, 1, false},
		{QualifyModeOnDemand, 2, true},
		{QualifyModeOnDemand, 3, true},
	}
	for _, tt := range tests {
		if got := tt.mode.Applies(tt.namespaces); got != tt.want {
			t.Errorf("%v.Applies(%d) = %v, want %v", tt.mode, tt.namespaces, got, tt.want)
		}
	}
}

// TestModeString pins the spelling a diagnostic or an error would use to the
// one the settings take, and that it parses back.
func TestQualifyModeString(t *testing.T) {
	all := QualifyModeSet{QualifyModeAlways, QualifyModeNever, QualifyModeOnDemand}
	for m, want := range map[QualifyMode]string{
		QualifyModeAlways:   "always",
		QualifyModeNever:    "never",
		QualifyModeOnDemand: "ondemand",
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
func TestQualifyModeSetParse(t *testing.T) {
	all := QualifyModeSet{QualifyModeAlways, QualifyModeNever, QualifyModeOnDemand}
	binary := QualifyModeSet{QualifyModeAlways, QualifyModeNever}
	tests := []struct {
		set   QualifyModeSet
		value string
		want  QualifyMode
		ok    bool
	}{
		{all, "always", QualifyModeAlways, true},
		{all, "never", QualifyModeNever, true},
		{all, "ondemand", QualifyModeOnDemand, true},
		{all, "true", 0, false},
		{all, "false", 0, false},
		{all, "sometimes", 0, false},
		{all, "", 0, false},
		{binary, "always", QualifyModeAlways, true},
		{binary, "never", QualifyModeNever, true},
		{binary, "ondemand", 0, false},
		{QualifyModeSet{QualifyModeNever}, "always", 0, false},
	}
	for _, tt := range tests {
		got, ok := tt.set.Parse(tt.value)
		if ok != tt.ok || got != tt.want {
			t.Errorf("%v.Parse(%v) = %v, %v; want %v, %v", tt.set, tt.value, got, ok, tt.want, tt.ok)
		}
	}
}

// TestModeSetString pins how a set names its members in an error message.
func TestQualifyModeSetString(t *testing.T) {
	tests := []struct {
		set  QualifyModeSet
		want string
	}{
		{QualifyModeSet{QualifyModeAlways, QualifyModeNever, QualifyModeOnDemand}, "always, never or ondemand"},
		{QualifyModeSet{QualifyModeAlways, QualifyModeNever}, "always or never"},
		{QualifyModeSet{QualifyModeNever}, "never"},
		{QualifyModeSet{}, ""},
	}
	for _, tt := range tests {
		if got := tt.set.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}
