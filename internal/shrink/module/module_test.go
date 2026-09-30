package module

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIgnorePatternsMatch pins the go command's reading of an ignore
// directive: ./x names x at the root only, x names a directory x anywhere,
// and both compare whole path elements.
func TestIgnorePatternsMatch(t *testing.T) {
	ps := ignorePatterns{fromRoot: []string{slashed("gen")}, anywhere: []string{slashed("static"), slashed("web/assets")}}
	for rel, want := range map[string]bool{
		"gen":                 true,
		"gen/sub":             true,
		"internal/gen":        false,
		"generated":           false,
		"static":              true,
		"web/static":          true,
		"web/static/css":      true,
		"nostatic":            false,
		"static2":             false,
		"web/assets":          true,
		"site/web/assets/img": true,
		"web":                 false,
	} {
		if got := ps.matches(rel); got != want {
			t.Errorf("matches(%q) = %v, want %v", rel, got, want)
		}
	}
}

// TestIgnoreDirectivesRefuses pins that a go.mod the walk cannot read stops
// the run rather than reading every ignored directory.
func TestIgnoreDirectivesRefuses(t *testing.T) {
	missing := t.TempDir()
	if _, err := ignoreDirectives(missing); err == nil {
		t.Error("a directory with no go.mod gave no error")
	}
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, "go.mod"), []byte("module\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ignoreDirectives(broken); err == nil {
		t.Error("a go.mod that does not parse gave no error")
	}
}
