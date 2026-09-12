package baseline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal/baseline"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".declscope-baseline.yaml")
	keys := []baseline.Key{
		{Package: "example.com/b", Rule: "boundary", Decl: "zeta"},
		{Package: "example.com/a", Rule: "boundary", Decl: "helper"},
		{Package: "example.com/a", Rule: "boundary", Decl: "User.name"},
		{Package: "example.com/a", Rule: "foreign-method", Decl: "User.normalize"},
		// A duplicate, as produced by a package and its test variant.
		{Package: "example.com/a", Rule: "boundary", Decl: "helper"},
	}
	n, err := baseline.Save(path, keys)
	if err != nil {
		t.Fatal(err)
	}
	// The count Save returns is what the subcommand reports, so it has to be
	// the number of entries in the file, not the number of keys handed in.
	if n != 4 {
		t.Errorf("Save returned %d, want 4 (the deduplicated count)", n)
	}

	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != 4 {
		t.Errorf("Len = %d, want 4 (duplicates collapse)", set.Len())
	}
	for _, k := range keys {
		if !set.Has(k) {
			t.Errorf("Has(%+v) = false", k)
		}
	}
	for _, k := range []baseline.Key{
		{Package: "example.com/a", Rule: "boundary", Decl: "missing"},
		{Package: "example.com/a", Rule: "demotion", Decl: "helper"},
		{Package: "example.com/other", Rule: "boundary", Decl: "helper"},
	} {
		if set.Has(k) {
			t.Errorf("Has(%+v) = true, want false", k)
		}
	}
}

// TestSaveIsDeterministic checks that regenerating an unchanged codebase
// produces no diff, which is what makes the file reviewable.
func TestSaveIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	keys := []baseline.Key{
		{Package: "example.com/b", Rule: "boundary", Decl: "zeta"},
		{Package: "example.com/a", Rule: "boundary", Decl: "helper"},
		{Package: "example.com/a", Rule: "boundary", Decl: "alpha"},
	}
	shuffled := []baseline.Key{keys[1], keys[0], keys[2]}

	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if _, err := baseline.Save(first, keys); err != nil {
		t.Fatal(err)
	}
	if _, err := baseline.Save(second, shuffled); err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("output depends on input order:\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
}

// TestLoadMissingFile checks that a configured but not yet generated baseline
// behaves as none rather than failing the run.
func TestLoadMissingFile(t *testing.T) {
	set, err := baseline.Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("a missing baseline should not be an error: %v", err)
	}
	if set.Len() != 0 {
		t.Errorf("Len = %d, want 0", set.Len())
	}
}

func TestLoadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := baseline.Load(path); err != nil {
		t.Fatalf("an empty baseline is valid: %v", err)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("packagez:\n  a:\n    boundary: [x]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := baseline.Load(path); err == nil {
		t.Fatal("a misspelled key must not silently suppress nothing")
	}
}

// TestLoadRejectsFormerRuleNames pins the decision on a baseline written before
// the rules were renamed: a section under escape, promote or demote is refused
// rather than read as boundary, qualify or unqualify. Left alone it would
// match nothing and every violation it recorded would come back with nothing
// saying why; the error instead names the rename and the remedy, which is the
// same regeneration that fixes every other baseline problem and never reads
// the file it replaces.
func TestLoadRejectsFormerRuleNames(t *testing.T) {
	for old, now := range map[string]string{
		"escape":  "boundary",
		"promote": "qualify",
		"demote":  "unqualify",
	} {
		path := filepath.Join(t.TempDir(), ".declscope-baseline.yaml")
		content := "packages:\n  example.com/a:\n    " + old + ": [helper]\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		set, err := baseline.Load(path)
		if err == nil {
			t.Errorf("%s: want an error, got a set of %d", old, set.Len())
			continue
		}
		for _, want := range []string{path, "example.com/a", old, now, "declscope baseline"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q does not mention %q", old, err, want)
			}
		}
	}
}

// TestNilSet checks that the zero value behaves as an absent baseline, which
// is how the analyzer runs when none is configured.
func TestNilSet(t *testing.T) {
	var set *baseline.Set
	if set.Has(baseline.Key{Package: "a", Rule: "boundary", Decl: "x"}) {
		t.Error("a nil set should suppress nothing")
	}
	if set.Len() != 0 {
		t.Error("a nil set should be empty")
	}
}

// TestSaveEmpty checks that a run with nothing to record still writes a
// well-formed file: regeneration prunes by rewriting, so an empty result must
// replace the old entries rather than leave them.
func TestSaveEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.yaml")
	if _, err := baseline.Save(path, []baseline.Key{{Package: "a", Rule: "boundary", Decl: "x"}}); err != nil {
		t.Fatal(err)
	}
	n, err := baseline.Save(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("Save returned %d, want 0", n)
	}
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if set.Len() != 0 {
		t.Errorf("Len = %d, want 0: the previous entries should be gone", set.Len())
	}
}
