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
  exported: package
  unexported: package
rules:
  qualify: false
  unqualify: true
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

	if opts.Exported != scope.PackageInternal || opts.Unexported != scope.PackageInternal {
		t.Errorf("defaults not applied: %+v", opts)
	}
	if opts.Qualify != internal.QualifyNever || !opts.CheckUnqualify {
		t.Errorf("rules not applied: %+v", opts)
	}
	if len(opts.Exclude) != 1 || opts.Exclude[0] != "**/mock_*.go" {
		t.Errorf("exclude not applied: %v", opts.Exclude)
	}
}

// TestApplyKeepsDefaults checks that omitting a key keeps the built-in
// default rather than resetting it to the zero value.
func TestApplyKeepsDefaults(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "rules:\n  unqualify: true\n")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := internal.DefaultOptions()
	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.Exported != want.Exported || opts.Unexported != want.Unexported {
		t.Errorf("defaults should be untouched, got %+v", opts)
	}
	if opts.Qualify != want.Qualify {
		t.Errorf("unnamed rules should be untouched, got %+v", opts)
	}
	if !opts.CheckUnqualify {
		t.Error("unqualify should be enabled")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "")
	if _, err := config.Load(path); err != nil {
		t.Fatalf("an empty config is valid and changes nothing: %v", err)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "rules:\n  unqualifyy: true\n")
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

// TestQualifyModes checks every spelling of the tri-state rules.qualify
// setting: the documented always, never and ondemand, and the true and false
// aliases, which YAML hands over as booleans unless quoted.
func TestQualifyModes(t *testing.T) {
	tests := []struct {
		yaml string
		want internal.QualifyMode
	}{
		{"rules:\n  qualify: always\n", internal.QualifyAlways},
		{"rules:\n  qualify: never\n", internal.QualifyNever},
		{"rules:\n  qualify: ondemand\n", internal.QualifyOnDemand},
		{"rules:\n  qualify: true\n", internal.QualifyAlways},
		{"rules:\n  qualify: false\n", internal.QualifyNever},
		{"rules:\n  qualify: \"true\"\n", internal.QualifyAlways},
		{"rules:\n  qualify: \"false\"\n", internal.QualifyNever},
	}
	for _, tt := range tests {
		path := write(t, t.TempDir(), ".declscope.yaml", tt.yaml)
		f, err := config.Load(path)
		if err != nil {
			t.Fatalf("%q: %v", tt.yaml, err)
		}
		opts := internal.DefaultOptions()
		if err := f.Apply(&opts); err != nil {
			t.Fatalf("%q: %v", tt.yaml, err)
		}
		if opts.Qualify != tt.want {
			t.Errorf("%q: Prefix = %v, want %v", tt.yaml, opts.Qualify, tt.want)
		}
	}
}

// TestQualifyModeString pins the spelling a diagnostic or an error would use
// to the documented one, and that it parses back: a mode printed as "true"
// would tell the reader to write a value the docs no longer show.
func TestQualifyModeString(t *testing.T) {
	for mode, want := range map[internal.QualifyMode]string{
		internal.QualifyAlways:   "always",
		internal.QualifyNever:    "never",
		internal.QualifyOnDemand: "ondemand",
	} {
		if got := mode.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
		if back, ok := internal.ParseQualifyMode(mode.String()); !ok || back != mode {
			t.Errorf("ParseQualifyMode(%q) = %v, %v; want %v", mode.String(), back, ok, mode)
		}
	}
}

// TestDefaultQualifyMode pins the default: the label is required only once a
// package has a second namespace to distinguish.
func TestDefaultQualifyMode(t *testing.T) {
	if got := internal.DefaultOptions().Qualify; got != internal.QualifyOnDemand {
		t.Errorf("default Prefix = %v, want ondemand", got)
	}
}

func TestApplyRejectsUnknownQualifyMode(t *testing.T) {
	path := write(t, t.TempDir(), ".declscope.yaml", "rules:\n  qualify: sometimes\n")
	f, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err == nil {
		t.Fatal("want an error for an unknown prefix mode")
	}
}

// TestApplyRejectsFormerRuleKeys pins the decision on the rules' former names:
// rules.promote and rules.demote are refused, not read as aliases of
// rules.qualify and rules.unqualify, and the error names the key to write.
// Unknown keys are already an error, so the alternative to an alias was never
// "ignore it"; it was this message or a bare "field not found".
func TestApplyRejectsFormerRuleKeys(t *testing.T) {
	for _, tt := range []struct {
		yaml string
		want string
	}{
		{"rules:\n  promote: always\n", "rules.qualify"},
		{"rules:\n  promote: true\n", "rules.qualify"},
		{"rules:\n  demote: true\n", "rules.unqualify"},
		{"rules:\n  demote: false\n", "rules.unqualify"},
	} {
		f, err := config.Load(write(t, t.TempDir(), ".declscope.yaml", tt.yaml))
		if err != nil {
			t.Fatalf("%q: Load must get past the decoder so the error can name the new key: %v", tt.yaml, err)
		}
		opts := internal.DefaultOptions()
		err = f.Apply(&opts)
		if err == nil {
			t.Errorf("%q: want an error naming %s, got none", tt.yaml, tt.want)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: error %q does not name %s", tt.yaml, err, tt.want)
		}
		if opts.Qualify != internal.QualifyOnDemand || opts.CheckUnqualify {
			t.Errorf("%q: a refused key must not change the options", tt.yaml)
		}
	}
}

// TestResolveRejectsFormerRuleKey checks the same through the lookup the
// analyzer uses, so that the path of the offending config is part of the
// message.
func TestResolveRejectsFormerRuleKey(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	path := write(t, root, ".declscope.yaml", "rules:\n  demote: true\n")

	_, _, err := config.Resolve(root, "")
	if err == nil {
		t.Fatal("want an error for rules.demote")
	}
	for _, want := range []string{path, "rules.unqualify"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

// TestResolveLoadsBaseline pins the analyzer's side of the lookup: a
// default-named baseline above the package is found and loaded.
func TestResolveLoadsBaseline(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope-baseline.yaml", "packages:\n  example.com/m/pkg:\n    boundary: [helper]\n")
	pkg := filepath.Join(root, "pkg")
	write(t, pkg, "keep.go", "package pkg\n")

	opts, _, err := config.Resolve(pkg, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, ".declscope-baseline.yaml"); opts.BaselinePath != want {
		t.Errorf("BaselinePath = %q, want %q", opts.BaselinePath, want)
	}
	if !opts.Baseline.Has(baseline.Key{Package: "example.com/m/pkg", Rule: "boundary", Decl: "helper"}) {
		t.Error("the baseline should be loaded")
	}
}

// TestResolveForBaselineIgnoresCorruptBaseline is the regeneration side: a
// baseline that fails to parse fails analysis, as it should, but must not
// stop the subcommand from replacing it — regenerating is the documented fix.
func TestResolveForBaselineIgnoresCorruptBaseline(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope-baseline.yaml", "packages:\n  x:\n    boundary: [helper]\nbogus: 1\n")
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
	write(t, sub, ".declscope.yaml", "baseline: sub-baseline.yaml\nrules:\n  unqualify: true\n")
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
	if !opts.CheckUnqualify {
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
