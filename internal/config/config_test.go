package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/config"
	"github.com/mpyw/declscope/internal/scope"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndApply(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", `
defaults:
  exported: package
  unexported: package
  prefixed: public
rules:
  members: false
  foreign-methods: true
  demotion: true
exclude:
  - "**/mock_*.go"
`)
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err != nil {
		t.Fatal(err)
	}

	if opts.Exported != scope.PackageInternal || opts.Unexported != scope.PackageInternal || opts.Prefixed != scope.Public {
		t.Errorf("defaults not applied: %+v", opts)
	}
	if opts.CheckMembers || !opts.CheckForeignMethods || !opts.CheckDemotion {
		t.Errorf("rules not applied: %+v", opts)
	}
	if len(opts.Exclude) != 1 || opts.Exclude[0] != "**/mock_*.go" {
		t.Errorf("exclude not applied: %v", opts.Exclude)
	}
}

// TestApplyKeepsDefaults checks that omitting a key keeps the built-in
// default rather than resetting it to the zero value.
func TestApplyKeepsDefaults(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "rules:\n  demotion: true\n")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := internal.DefaultOptions()
	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.Exported != want.Exported || opts.Unexported != want.Unexported || opts.Prefixed != want.Prefixed {
		t.Errorf("defaults should be untouched, got %+v", opts)
	}
	if opts.CheckMembers != want.CheckMembers {
		t.Errorf("members should be untouched, got %v", opts.CheckMembers)
	}
	if !opts.CheckDemotion {
		t.Error("demotion should be enabled")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "")
	if _, err := config.Load(path); err != nil {
		t.Fatalf("an empty config is valid and changes nothing: %v", err)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "rules:\n  memberz: true\n")
	if _, err := config.Load(path); err == nil {
		t.Fatal("a misspelled rule must not silently keep its default")
	}
}

func TestApplyRejectsUnknownScope(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "defaults:\n  unexported: private\n")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err == nil {
		t.Fatal("want an error for an unknown scope name")
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml", "")
	write(t, filepath.Join(root, "pkg", "sub"), "keep.go", "package sub\n")

	got := config.Find(filepath.Join(root, "pkg", "sub"))
	if want := filepath.Join(root, ".declscope.yaml"); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

// TestFindStopsAtModuleRoot checks that a config file above the module cannot
// silently change the module's rules.
func TestFindStopsAtModuleRoot(t *testing.T) {
	outer := t.TempDir()
	write(t, outer, ".declscope.yaml", "")
	root := filepath.Join(outer, "m")
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, filepath.Join(root, "pkg"), "keep.go", "package pkg\n")

	if got := config.Find(filepath.Join(root, "pkg")); got != "" {
		t.Errorf("Find = %q, want \"\"", got)
	}
}

func TestFindPrefersNearest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml", "")
	nested := filepath.Join(root, "legacy")
	write(t, nested, ".declscope.yml", "")

	got := config.Find(nested)
	if want := filepath.Join(nested, ".declscope.yml"); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}
