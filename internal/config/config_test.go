package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/baseline"
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
  unexported: package
rules:
  naming:
    qualify: never
    exported: true
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

	if opts.Unexported != scope.PackageInternal {
		t.Errorf("defaults not applied: %+v", opts)
	}
	if opts.Qualify != internal.ModeNever || !opts.NameExported {
		t.Errorf("rules not applied: %+v", opts)
	}
	if len(opts.Exclude) != 1 || opts.Exclude[0] != "**/mock_*.go" {
		t.Errorf("exclude not applied: %v", opts.Exclude)
	}
}

// TestApplyKeepsDefaults checks that omitting a key keeps the built-in
// default rather than resetting it to the zero value.
func TestApplyKeepsDefaults(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "rules:\n  naming:\n    exported: true\n")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := internal.DefaultOptions()
	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.Unexported != want.Unexported {
		t.Errorf("defaults should be untouched, got %+v", opts)
	}
	if opts.Qualify != want.Qualify {
		t.Errorf("unnamed rules should be untouched, got %+v", opts)
	}
	if !opts.NameExported {
		t.Error("exported should be enabled")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "")
	if _, err := config.Load(path); err != nil {
		t.Fatalf("an empty config is valid and changes nothing: %v", err)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	for _, tt := range []struct{ yaml, want string }{
		{"nonsense: 1\n", `unknown key "nonsense" (this section takes defaults, rules, exclude, baseline)`},
		{"rules:\n  unqualifyy: always\n", `unknown key "rules.unqualifyy" (this section takes naming)`},
		{"rules:\n  naming:\n    unqualifyy: always\n", `unknown key "rules.naming.unqualifyy" (this section takes qualify, exported)`},
		// The removed rule is refused through the same path as any typo, so a
		// config written for the release that had it fails loudly rather than
		// silently dropping the setting.
		{"rules:\n  naming:\n    unqualify: true\n", `unknown key "rules.naming.unqualify" (this section takes qualify, exported)`},
		{"defaults:\n  unexpected: private\n", `unknown key "defaults.unexpected" (this section takes unexported)`},
	} {
		path := write(t, t.TempDir(), ".declscope.yaml", tt.yaml)
		_, err := config.Load(path)
		if err == nil {
			t.Fatalf("%q: a misspelled key must not silently keep the default", tt.yaml)
		}
		// The Go type go-yaml names is an implementation detail, and for a
		// nested section it is the whole struct literal.
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: got %v, want it to contain %s", tt.yaml, err, tt.want)
		}
	}
}

// TestApplyRejectsUnknownScope checks that an unknown scope value is refused
// with an error naming what is accepted.
func TestApplyRejectsUnknownScope(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "defaults:\n  unexported: file\n")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := internal.DefaultOptions()
	err = f.Apply(&opts)
	if err == nil {
		t.Fatal("want an error for an unknown scope name")
	}
	if want := "package or private"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not name the accepted values %q", err, want)
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
	nested := filepath.Join(root, "store")
	write(t, nested, ".declscope.yml", "")

	got := config.Find(nested)
	if want := filepath.Join(nested, ".declscope.yml"); got != want {
		t.Errorf("Find = %q, want %q", got, want)
	}
}

// apply loads a config body and layers it onto the defaults.
// settingErr returns whichever stage rejected the config. A value a key does
// not accept is caught while decoding when the key is a boolean and while
// applying when it is a mode, and a caller checking the message should not have
// to know which.
func settingErr(t *testing.T, body string) error {
	t.Helper()
	path := write(t, t.TempDir(), ".declscope.yaml", body)
	f, err := config.Load(path)
	if err != nil {
		return err
	}
	opts := internal.DefaultOptions()
	return f.Apply(&opts)
}

func apply(t *testing.T, body string) (internal.Options, error) {
	t.Helper()
	path := write(t, t.TempDir(), ".declscope.yaml", body)
	f, err := config.Load(path)
	if err != nil {
		t.Fatalf("%q: %v", body, err)
	}
	opts := internal.DefaultOptions()
	err = f.Apply(&opts)
	return opts, err
}

// TestQualifyModes checks every value of the rules.qualify setting.
func TestQualifyModes(t *testing.T) {
	tests := []struct {
		yaml string
		want internal.Mode
	}{
		{"rules:\n  naming:\n    qualify: always\n", internal.ModeAlways},
		{"rules:\n  naming:\n    qualify: never\n", internal.ModeNever},
		{"rules:\n  naming:\n    qualify: ondemand\n", internal.ModeOnDemand},
	}
	for _, tt := range tests {
		opts, err := apply(t, tt.yaml)
		if err != nil {
			t.Fatalf("%q: %v", tt.yaml, err)
		}
		if opts.Qualify != tt.want {
			t.Errorf("%q: Qualify = %v, want %v", tt.yaml, opts.Qualify, tt.want)
		}
	}
}

// TestBoolSettings checks both values of every boolean setting.
func TestBoolSettings(t *testing.T) {
	tests := []struct {
		yaml string
		get  func(internal.Options) bool
		want bool
	}{
		{"rules:\n  naming:\n    exported: true\n", func(o internal.Options) bool { return o.NameExported }, true},
		{"rules:\n  naming:\n    exported: false\n", func(o internal.Options) bool { return o.NameExported }, false},
	}
	for _, tt := range tests {
		opts, err := apply(t, tt.yaml)
		if err != nil {
			t.Fatalf("%q: %v", tt.yaml, err)
		}
		if got := tt.get(opts); got != tt.want {
			t.Errorf("%q: got %v, want %v", tt.yaml, got, tt.want)
		}
	}
}

// TestDefaultModes pins the defaults: the naming rule is off until a config
// asks for it, since its findings did not correlate with boundary violations
// on the repositories measured.
func TestDefaultModes(t *testing.T) {
	opts := internal.DefaultOptions()
	if opts.Qualify != internal.ModeNever {
		t.Errorf("default Qualify = %v, want never", opts.Qualify)
	}
	if opts.NameExported {
		t.Error("default NameExported should be off")
	}
}

// TestApplyRejectsUnknownMode checks that a value a setting does not accept
// is refused with the values it does. A YAML bool is one such value: the
// settings take words only.
func TestApplyRejectsUnknownMode(t *testing.T) {
	tests := []struct {
		yaml string
		want string
	}{
		{"rules:\n  naming:\n    qualify: sometimes\n",
			`rules.naming.qualify: unknown mode "sometimes" (want always, never or ondemand)`},
		{"rules:\n  naming:\n    qualify: true\n",
			`rules.naming.qualify: unknown mode "true" (want always, never or ondemand)`},
		{"rules:\n  naming:\n    qualify: false\n",
			`rules.naming.qualify: unknown mode "false" (want always, never or ondemand)`},
	}
	for _, tt := range tests {
		_, err := apply(t, tt.yaml)
		if err == nil {
			t.Fatalf("%q: want an error for an unknown mode", tt.yaml)
		}
		if got := err.Error(); got != tt.want {
			t.Errorf("%q: error = %q, want %q", tt.yaml, got, tt.want)
		}
	}
}

// TestBoolSettingRejectsAWord checks that a boolean setting refuses a word,
// naming what it takes.
func TestBoolSettingRejectsAWord(t *testing.T) {
	for _, yaml := range []string{"rules:\n  naming:\n    exported: ondemand\n"} {
		err := settingErr(t, yaml)
		if err == nil {
			t.Fatalf("%q: want an error", yaml)
		}
		if !strings.Contains(err.Error(), "want true or false") {
			t.Errorf("%q: error %q does not say what is accepted", yaml, err)
		}
	}
}

// TestResolveLoadsBaseline pins the analyzer's side of the lookup: a
// default-named baseline above the package is found and loaded.
func TestResolveLoadsBaseline(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope-baseline.yaml", "packages:\n  example.com/m/pkg:\n    boundary:\n      user: [helper]\n")
	pkg := filepath.Join(root, "pkg")
	write(t, pkg, "keep.go", "package pkg\n")

	opts, _, err := config.Resolve(pkg, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, ".declscope-baseline.yaml"); opts.BaselinePath != want {
		t.Errorf("BaselinePath = %q, want %q", opts.BaselinePath, want)
	}
	if !opts.Baseline.Has(baseline.Key{Package: "example.com/m/pkg", Rule: "boundary", Namespace: "user", Decl: "helper"}) {
		t.Error("the baseline should be loaded")
	}
}

// TestResolveForBaselineIgnoresCorruptBaseline is the regeneration side: a
// baseline that fails to parse fails analysis, as it should, but must not
// stop the subcommand from replacing it — regenerating is the documented fix.
func TestResolveForBaselineIgnoresCorruptBaseline(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope-baseline.yaml", "packages:\n  x:\n    boundary:\n      user: [helper]\nbogus: 1\n")
	pkg := filepath.Join(root, "pkg")
	write(t, pkg, "keep.go", "package pkg\n")

	if _, _, err := config.Resolve(pkg, ""); err == nil {
		t.Fatal("Resolve should refuse a baseline it cannot parse")
	}
	opts, _, named, err := config.ResolveForBaseline(pkg, "")
	if err != nil {
		t.Fatalf("ResolveForBaseline must not load the baseline: %v", err)
	}
	if named != "" {
		t.Errorf("named = %q, want \"\": no config names one", named)
	}
	if opts.BaselinePath != "" || opts.Baseline != nil {
		t.Errorf("no baseline should be attached to the options, got %q / %v", opts.BaselinePath, opts.Baseline)
	}
}

// TestResolveForBaselineReturnsNamed checks that the configured baseline is
// handed back as the place to write, resolved against the config file, and
// that its content — corrupt here — is never read.
func TestResolveForBaselineReturnsNamed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	sub := filepath.Join(root, "sub")
	write(t, sub, ".declscope.yaml", "baseline: sub-baseline.yaml\nrules:\n  naming:\n    exported: true\n")
	write(t, sub, "sub-baseline.yaml", "not: [valid\n")
	inner := filepath.Join(sub, "inner")
	write(t, inner, "keep.go", "package inner\n")

	opts, configPath, named, err := config.ResolveForBaseline(inner, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(sub, ".declscope.yaml"); configPath != want {
		t.Errorf("config = %q, want %q", configPath, want)
	}
	if want := filepath.Join(sub, "sub-baseline.yaml"); named != want {
		t.Errorf("named = %q, want %q", named, want)
	}
	if !opts.NameExported {
		t.Error("the rest of the config should still apply")
	}
	if opts.Baseline != nil {
		t.Error("the baseline must not be loaded during regeneration")
	}
}

// TestDefaultBaseline pins where a package's entries go when no config names
// a baseline. The invariant is that the analyzer, walking up from the
// package, finds exactly the file the generator wrote.
func TestDefaultBaseline(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, filepath.Join(root, "a"), "keep.go", "package a\n")
	write(t, filepath.Join(root, "b", "inner"), "keep.go", "package inner\n")
	write(t, filepath.Join(root, "b"), ".declscope-baseline.yml", "")
	write(t, filepath.Join(root, "nested"), "go.mod", "module example.com/nested\n")
	write(t, filepath.Join(root, "nested", "pkg"), "keep.go", "package pkg\n")
	write(t, filepath.Join(root, "store", "inner"), "keep.go", "package inner\n")
	write(t, root, ".declscope-baseline.yaml", "")

	rootDefault := filepath.Join(root, ".declscope-baseline.yaml")
	tests := []struct {
		name, dir, from string
		want            string
		ok              bool
	}{
		{"new file in the working directory", filepath.Join(root, "a"), root, rootDefault, true},
		{"the working directory itself", root, root, rootDefault, true},
		{"nearest existing file, shadowing the root", filepath.Join(root, "b", "inner"), root, filepath.Join(root, "b", ".declscope-baseline.yml"), true},
		// Run from store/: the root file exists, but a run that never saw the
		// packages outside store/ must not rewrite it.
		{"stops at the working directory", filepath.Join(root, "store", "inner"), filepath.Join(root, "store"), filepath.Join(root, "store", ".declscope-baseline.yaml"), true},
		{"module boundary before the working directory", filepath.Join(root, "nested", "pkg"), root, "", false},
		{"package outside the working directory", filepath.Join(root, "a"), filepath.Join(root, "b"), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := config.DefaultBaseline(tt.dir, tt.from)
			if ok != tt.ok || got != tt.want {
				t.Errorf("DefaultBaseline(%q, %q) = %q, %v; want %q, %v", tt.dir, tt.from, got, ok, tt.want, tt.ok)
			}
			if !ok {
				return
			}
			// Once written, the analyzer's lookup from the package must land
			// on the same file.
			if !fileExists(got) {
				write(t, filepath.Dir(got), filepath.Base(got), "")
				t.Cleanup(func() { _ = os.Remove(got) })
			}
			if found := config.FindBaseline(tt.dir); found != got {
				t.Errorf("FindBaseline(%q) = %q after writing, want %q", tt.dir, found, got)
			}
		})
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
