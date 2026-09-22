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
    vocabulary:
      mouse: [wheel, cursor]
      index: [indices]
filter:
  only:
    - "internal/**"
  omit:
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
	if opts.Qualify != internal.QualifyModeNever || !opts.NameExported {
		t.Errorf("rules not applied: %+v", opts)
	}
	// The pattern arrives as written, together with the directory it is to be
	// read against, so that it means the same thing wherever the config moves.
	// One stating file makes one only group, and every pattern carries the
	// directory it was written in, so a chain can hold several depths.
	if len(opts.Only) != 1 || len(opts.Only[0]) != 1 ||
		opts.Only[0][0] != (internal.FilterPattern{Pattern: "internal/**", Base: filepath.Dir(path)}) {
		t.Errorf("filter.only not applied: %v", opts.Only)
	}
	if len(opts.Omit) != 1 ||
		opts.Omit[0] != (internal.FilterPattern{Pattern: "**/mock_*.go", Base: filepath.Dir(path)}) {
		t.Errorf("filter.omit not applied: %v", opts.Omit)
	}
	if len(opts.Vocabulary["mouse"]) != 2 || opts.Vocabulary["mouse"][0] != "wheel" ||
		len(opts.Vocabulary["index"]) != 1 || opts.Vocabulary["index"][0] != "indices" {
		t.Errorf("vocabulary not applied: %v", opts.Vocabulary)
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
		{"nonsense: 1\n", `unknown key "nonsense" (this section takes defaults, rules, filter, baseline)`},
		{"rules:\n  unqualifyy: always\n", `unknown key "rules.unqualifyy" (this section takes naming, allowBoundary, surplus)`},
		// The switch rules.surplus replaced is refused the same way, and the
		// message names the key that replaced it.
		{"rules:\n  allowSurplus: true\n", `unknown key "rules.allowSurplus" (this section takes naming, allowBoundary, surplus)`},
		{"rules:\n  naming:\n    unqualifyy: always\n", `unknown key "rules.naming.unqualifyy" (this section takes qualify, exported, vocabulary)`},
		// The removed rule is refused through the same path as any typo, so a
		// config written for the release that had it fails loudly rather than
		// silently dropping the setting.
		{"rules:\n  naming:\n    unqualify: true\n", `unknown key "rules.naming.unqualify" (this section takes qualify, exported, vocabulary)`},
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

// TestLoadPassesOtherTypeErrorsThrough checks the other half of the key
// naming. A YAML error that is not a misspelled key has nothing to rename, so
// it reaches the caller as go-yaml wrote it rather than being dropped in
// favour of a message about keys.
func TestLoadPassesOtherTypeErrorsThrough(t *testing.T) {
	for _, yaml := range []string{
		"rules:\n  allowBoundary: [1, 2]\n", // a list where a bool belongs
		"rules:\n  surplus: [1, 2]\n",       // a list where a mode belongs
		"rules:\n  naming:\n    vocabulary: 7\n",
	} {
		path := write(t, t.TempDir(), ".declscope.yaml", yaml)
		_, err := config.Load(path)
		if err == nil {
			t.Fatalf("%q: a value of the wrong shape must not be accepted", yaml)
		}
		if strings.Contains(err.Error(), "unknown key") {
			t.Errorf("%q: got %v, want the error go-yaml reported", yaml, err)
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
		want internal.QualifyMode
	}{
		{"rules:\n  naming:\n    qualify: always\n", internal.QualifyModeAlways},
		{"rules:\n  naming:\n    qualify: never\n", internal.QualifyModeNever},
		{"rules:\n  naming:\n    qualify: ondemand\n", internal.QualifyModeOnDemand},
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

// TestSurplusModes checks every value of the rules.surplus setting, and that
// each spells back the way the config does.
func TestSurplusModes(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  internal.SurplusMode
	}{
		{"off", internal.SurplusModeOff},
		{"loose", internal.SurplusModeLoose},
		{"strict", internal.SurplusModeStrict},
	} {
		opts, err := apply(t, "rules:\n  surplus: "+tt.value+"\n")
		if err != nil {
			t.Fatalf("%q: %v", tt.value, err)
		}
		if opts.Surplus != tt.want {
			t.Errorf("%q: Surplus = %v, want %v", tt.value, opts.Surplus, tt.want)
		}
		if got := opts.Surplus.String(); got != tt.value {
			t.Errorf("%q: String() = %q, want the config's own spelling", tt.value, got)
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
		{"rules:\n  allowBoundary: true\n", func(o internal.Options) bool { return o.AllowBoundary }, true},
		{"rules:\n  allowBoundary: false\n", func(o internal.Options) bool { return o.AllowBoundary }, false},
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
	if opts.Qualify != internal.QualifyModeNever {
		t.Errorf("default Qualify = %v, want never", opts.Qualify)
	}
	if opts.NameExported {
		t.Error("default NameExported should be off")
	}
	if opts.AllowBoundary {
		t.Error("the boundary rule has no switch to reach for, so AllowBoundary starts false")
	}
	// loose, not strict: an upgrade must not add reports to a repository
	// whose config did not change.
	if opts.Surplus != internal.SurplusModeLoose {
		t.Errorf("default Surplus = %v, want loose", opts.Surplus)
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
		// rules.surplus is answered with its own values, not qualify's.
		{"rules:\n  surplus: ondemand\n",
			`rules.surplus: unknown mode "ondemand" (want off, loose or strict)`},
		{"rules:\n  surplus: true\n",
			`rules.surplus: unknown mode "true" (want off, loose or strict)`},
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
	for _, yaml := range []string{
		"rules:\n  naming:\n    exported: ondemand\n",
		"rules:\n  allowBoundary: ondemand\n",
	} {
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

// TestLoadMakesPathAbsolute checks that a config named relative to the working
// directory still anchors its filter patterns. The patterns are read against
// the config's own directory, and the paths they are matched to come from the
// driver as absolute ones; a directory spelled "." would agree with none of
// them, and would skip nothing without reporting anything.
func TestLoadMakesPathAbsolute(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".declscope.yaml")
	if err := os.WriteFile(path, []byte("filter:\n  omit: [\"vendor/**\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	f, err := config.Load(".declscope.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(f.FilterBase()) {
		t.Fatalf("filter base = %q, want an absolute directory", f.FilterBase())
	}

	opts := internal.DefaultOptions()
	if err := f.Apply(&opts); err != nil {
		t.Fatal(err)
	}
	if err := opts.Compile(); err != nil {
		t.Fatal(err)
	}
	if !opts.Skips(filepath.Join(dir, "vendor", "foo.go")) {
		t.Error("a relatively named config should still anchor its patterns")
	}
}

// TestNestedConfigInheritsAndOverrides pins how two config files compose. The
// nearer one owns the keys it states, and every key it does not state comes
// from the file above rather than being dropped because a nearer file existed.
//
// The lookup used to stop at the first file, which meant a nested config that
// set one key silently lost the root's. baseline never worked that way — it
// has its own upward search — so this makes one key's behaviour the rule.
func TestNestedConfigInheritsAndOverrides(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml",
		"rules:\n  naming:\n    qualify: always\n    vocabulary:\n      mouse: [wheel]\n"+
			"filter:\n  omit:\n    - \"**/root_omitted.go\"\n")
	sub := filepath.Join(root, "sub")
	write(t, sub, ".declscope.yaml",
		"rules:\n  naming:\n    exported: true\n    vocabulary:\n      index: [indices]\n"+
			"filter:\n  omit:\n    - \"**/sub_omitted.go\"\n")

	if chain := config.FindChain(sub); len(chain) != 2 {
		t.Fatalf("FindChain(%q) = %v, want both files outermost first", sub, chain)
	}
	opts, _, err := config.Resolve(sub, "")
	if err != nil {
		t.Fatal(err)
	}

	// Stated nearest, so it wins.
	if !opts.NameExported {
		t.Error("the nested file states exported, so it should be on")
	}
	// Stated only at the root, so it survives a nested file that is silent.
	if opts.Qualify != internal.QualifyModeAlways {
		t.Errorf("qualify = %v, want the root's, which the nested file did not restate", opts.Qualify)
	}
	// The map merges per namespace rather than the nearer one replacing it.
	if len(opts.Vocabulary["mouse"]) != 1 || len(opts.Vocabulary["index"]) != 1 {
		t.Errorf("vocabulary = %v, want both namespaces", opts.Vocabulary)
	}
	// omit unions, so an omit written at the root still holds below it.
	if !opts.Skips(filepath.Join(sub, "root_omitted.go")) {
		t.Error("the root's omit should reach a package a nested config governs")
	}
	if !opts.Skips(filepath.Join(sub, "sub_omitted.go")) {
		t.Error("the nested omit should apply too")
	}
	if opts.Skips(filepath.Join(sub, "ordinary.go")) {
		t.Error("a file neither level names should be read")
	}
}

// TestSurplusModeComposes checks that rules.surplus composes like every other
// key: a nested file that states it wins, one that is silent keeps the
// root's, and it moves no other switch.
func TestSurplusModeComposes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml", "rules:\n  surplus: strict\n")
	silent := filepath.Join(root, "silent")
	write(t, silent, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
	stated := filepath.Join(root, "stated")
	write(t, stated, ".declscope.yaml", "rules:\n  surplus: off\n")

	opts, _, err := config.Resolve(silent, "")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Surplus != internal.SurplusModeStrict {
		t.Errorf("Surplus = %v: a nested file that does not state it should keep the root's", opts.Surplus)
	}
	if opts.AllowBoundary {
		t.Error("rules.surplus must switch no other rule")
	}
	opts, _, err = config.Resolve(stated, "")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Surplus != internal.SurplusModeOff {
		t.Errorf("Surplus = %v: a nested file that states it should win", opts.Surplus)
	}
}

// TestNestedOnlyCanOnlyNarrow checks the other half of the chain. only groups
// intersect, so a nested file may narrow what the root admits and can never
// bring back a file the root left out.
func TestNestedOnlyCanOnlyNarrow(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml", "filter:\n  only:\n    - \"keep/**\"\n")
	sub := filepath.Join(root, "keep", "sub")
	write(t, sub, ".declscope.yaml", "filter:\n  only:\n    - \"deep/**\"\n")

	opts, _, err := config.Resolve(sub, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.Only) != 2 {
		t.Fatalf("only = %v, want one group per stating file", opts.Only)
	}
	// Inside both groups.
	if opts.Skips(filepath.Join(sub, "deep", "a.go")) {
		t.Error("a file both levels admit should be read")
	}
	// The nested group narrows further.
	if !opts.Skips(filepath.Join(sub, "shallow", "a.go")) {
		t.Error("the nested only should narrow what the root admitted")
	}
	// And it cannot widen: a path outside the root's only stays out, however
	// the nested group is spelled.
	if !opts.Skips(filepath.Join(root, "elsewhere", "deep", "a.go")) {
		t.Error("a nested only must not bring back what the root left out")
	}
}

// TestNestedConfigAnchorsToItsOwnDirectory checks that each level's patterns
// are read against that level's directory. The same line therefore names a
// different place in each file, which is the rule a .gitignore follows, and
// the reason a chain carries a base per pattern rather than one for all.
func TestNestedConfigAnchorsToItsOwnDirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml", "filter:\n  omit:\n    - \"gen/**\"\n")
	sub := filepath.Join(root, "sub")
	write(t, sub, ".declscope.yaml", "filter:\n  omit:\n    - \"gen/**\"\n")

	opts, _, err := config.Resolve(sub, "")
	if err != nil {
		t.Fatal(err)
	}
	// The same spelling in two files names two directories, and both hold.
	if !opts.Skips(filepath.Join(root, "gen", "a.go")) {
		t.Error("the root's gen/** should name the directory beside the root config")
	}
	if !opts.Skips(filepath.Join(sub, "gen", "a.go")) {
		t.Error("the nested gen/** should name the directory beside the nested config")
	}
	// And neither reaches a third place.
	if opts.Skips(filepath.Join(root, "other", "gen", "a.go")) {
		t.Error("an anchored pattern should not float")
	}
}

// TestExplicitConfigDoesNotChain checks that -config names one file and gets
// one file. The caller said which rules to use, so nothing above is consulted.
func TestExplicitConfigDoesNotChain(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n")
	write(t, root, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
	sub := filepath.Join(root, "sub")
	explicit := write(t, sub, ".declscope.yaml", "rules:\n  naming:\n    exported: true\n")

	opts, _, err := config.Resolve(sub, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.NameExported {
		t.Error("the named file should apply")
	}
	if opts.Qualify != internal.QualifyModeNever {
		t.Errorf("qualify = %v, want the built-in default: an explicit config builds no chain", opts.Qualify)
	}
}
