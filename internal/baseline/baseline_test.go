package baseline_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mpyw/declscope/internal/baseline"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".declscope-baseline.yaml")
	keys := []baseline.Key{
		{Package: "example.com/b", Rule: "boundary", Namespace: "user", Decl: "zeta"},
		{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "helper"},
		{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "User.name"},
		{Package: "example.com/a", Rule: "foreign-method", Namespace: "user", Decl: "User.normalize"},
		// A duplicate, as produced by a package and its test variant.
		{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "helper"},
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
		{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "missing"},
		{Package: "example.com/a", Rule: "demotion", Namespace: "user", Decl: "helper"},
		{Package: "example.com/other", Rule: "boundary", Namespace: "user", Decl: "helper"},
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
		{Package: "example.com/b", Rule: "boundary", Namespace: "user", Decl: "zeta"},
		{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "helper"},
		{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "alpha"},
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

// TestNilSet checks that the zero value behaves as an absent baseline, which
// is how the analyzer runs when none is configured.
func TestNilSet(t *testing.T) {
	var set *baseline.Set
	if set.Has(baseline.Key{Package: "a", Rule: "boundary", Namespace: "user", Decl: "x"}) {
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
	if _, err := baseline.Save(path, []baseline.Key{{Package: "a", Rule: "boundary", Namespace: "user", Decl: "x"}}); err != nil {
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

// TestNamespaceIsPartOfTheKey checks that the same declaration in two
// namespaces is two entries.
//
// This is the whole reason the namespace is in the key. A declaration that
// moved to another file crosses a different pair of namespaces, so it is a
// different violation, and an entry that matched it anyway would suppress
// something nobody recorded.
func TestNamespaceIsPartOfTheKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".declscope-baseline.yaml")
	recorded := baseline.Key{Package: "example.com/a", Rule: "boundary", Namespace: "user", Decl: "helper"}
	moved := baseline.Key{Package: "example.com/a", Rule: "boundary", Namespace: "order", Decl: "helper"}
	if _, err := baseline.Save(path, []baseline.Key{recorded}); err != nil {
		t.Fatal(err)
	}
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Has(recorded) {
		t.Error("the recorded crossing is not suppressed")
	}
	if set.Has(moved) {
		t.Error("the same declaration in another namespace is a different crossing, and must not be suppressed")
	}
}

// TestCoreNamespaceRoundTrips checks the one namespace with no name of its
// own. It is spelled (core), which a normalized namespace cannot be, so a
// package holding both a core file and a core.go keeps them apart.
func TestCoreNamespaceRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".declscope-baseline.yaml")
	core := baseline.Key{Package: "example.com/a", Rule: "boundary", Namespace: "(core)", Decl: "dial"}
	named := baseline.Key{Package: "example.com/a", Rule: "boundary", Namespace: "core", Decl: "dial"}
	if _, err := baseline.Save(path, []baseline.Key{core}); err != nil {
		t.Fatal(err)
	}
	set, err := baseline.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !set.Has(core) {
		t.Error("the core namespace does not round-trip")
	}
	if set.Has(named) {
		t.Error("a namespace named core is not the core namespace")
	}
}

// TestLoadReportsAnUnreadableFile checks that only a missing file reads as an
// empty baseline. One that exists and cannot be read is an error, or a
// baseline that suppressed everything would silently suppress nothing.
func TestLoadReportsAnUnreadableFile(t *testing.T) {
	if _, err := baseline.Load(t.TempDir()); err == nil {
		t.Error("loading a directory as a baseline succeeded, want an error")
	}
}

// TestSaveReportsWhereItCannotWrite checks both ways a write can fail: the
// directory cannot be made, because a file stands where it would go, or the
// path itself is a directory.
func TestSaveReportsWhereItCannotWrite(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	keys := []baseline.Key{{Package: "p", Rule: "boundary", Namespace: "user", Decl: "helper"}}
	for name, path := range map[string]string{
		"a file in the way of the directory": filepath.Join(blocker, "sub", ".declscope-baseline.yaml"),
		"a directory at the path":            dir,
	} {
		if _, err := baseline.Save(path, keys); err == nil {
			t.Errorf("%s: Save succeeded, want an error", name)
		}
	}
}
