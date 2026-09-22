package scope_test

import (
	"testing"

	"github.com/mpyw/declscope/internal/scope"
)

// TestScopeSpellings pins the two names each level has: the directive that
// selects it, which a diagnostic offers as the remedy and a config file writes,
// and the prose name a message reads it back as. A level whose directive did
// not round-trip through Parse would be a scope nobody could state.
func TestScopeSpellings(t *testing.T) {
	tests := []struct {
		s         scope.Scope
		keyword   string
		directive string
		text      string
	}{
		{scope.PackageInternal, "package", "//declscope:package", "package-internal"},
		{scope.Private, "private", "//declscope:private", "private"},
	}
	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			if got := tt.s.Directive(); got != tt.directive {
				t.Errorf("Directive() = %q, want %q", got, tt.directive)
			}
			if got := tt.s.String(); got != tt.text {
				t.Errorf("String() = %q, want %q", got, tt.text)
			}
			got, ok := scope.Parse(tt.keyword)
			if !ok || got != tt.s {
				t.Errorf("Parse(%q) = %v, %v, want %v, true", tt.keyword, got, ok, tt.s)
			}
		})
	}
}

// TestScopeRejectsUnknown checks the other half: a keyword naming no level is
// refused rather than resolved to the zero value, which is package-internal
// and would silently widen whatever carried the typo.
func TestScopeRejectsUnknown(t *testing.T) {
	for _, keyword := range []string{"", "public", "Package", "internal"} {
		if got, ok := scope.Parse(keyword); ok {
			t.Errorf("Parse(%q) = %v, true, want false", keyword, got)
		}
	}
	// A value outside the enum renders rather than passing for a level.
	other := scope.Scope(42)
	if got := other.String(); got != "unknown" {
		t.Errorf("String() = %q, want %q", got, "unknown")
	}
	if got := other.Directive(); got != "" {
		t.Errorf("Directive() = %q, want an empty string", got)
	}
}
