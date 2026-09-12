package declscope_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The properties every suggested fix must satisfy, checked end to end through
// the real driver rather than through analysistest goldens:
//
//	1. the code still compiles after -fix
//	2. every diagnostic that carried a fix is gone
//	3. no diagnostic exists that did not exist before
//
// Three separate defects violating (2) or (3) were shipped and found by hand
// before this existed: a directive fix that collided with an existing
// directive and produced code the linter itself rejected, a directive placed
// on the wrong line for a field of a single-line struct, and a rename offered
// where a prefix could not widen anything. A golden file pins what a fix
// produces; only applying it and looking again pins that it helped.

type fixCase struct {
	name   string
	config string
	files  map[string]string
}

var fixCases = []fixCase{
	{
		name: "escape and promote on the same declaration",
		files: map[string]string{
			"user.go":  "package x\n\nfunc helper() int { return 1 }\n",
			"order.go": "package x\n\nfunc orderRun() int { return helper() }\n\nvar _ = orderRun\n",
		},
	},
	{
		name: "field of a single-line struct",
		files: map[string]string{
			"user.go":  "package x\n\ntype User struct{ name string }\n",
			"order.go": "package x\n\nfunc orderRun(u *User) string { return u.name }\n\nvar _ = orderRun\n",
		},
	},
	{
		name: "declaration already carrying a doc comment",
		files: map[string]string{
			"user.go":  "package x\n\n// userCache caches.\nfunc userCache() int { return 1 }\n",
			"order.go": "package x\n\nfunc orderRun() int { return userCache() }\n\nvar _ = orderRun\n",
		},
	},
	{
		name: "grouped var and const blocks",
		files: map[string]string{
			"user.go":  "package x\n\nvar (\n\tseed  = 1\n\tother = 2\n)\n\nconst (\n\tlimit = 3\n)\n",
			"order.go": "package x\n\nfunc orderRun() int { return seed + other + limit }\n\nvar _ = orderRun\n",
		},
	},
	{
		name: "rename target already taken",
		files: map[string]string{
			"user.go":  "package x\n\nfunc taken() int { return 1 }\n\nfunc userTaken() int { return 2 }\n",
			"order.go": "package x\n\nfunc orderRun() int { return taken() + userTaken() }\n\nvar _ = orderRun\n",
		},
	},
	{
		name: "method on a type belonging to another namespace",
		files: map[string]string{
			"user.go":  "package x\n\ntype User struct{ ID int }\n",
			"order.go": "package x\n\nfunc (u *User) bump() { u.ID++ }\n\nfunc OrderRun(u *User) { u.bump() }\n",
		},
	},
	{
		name:   "demote, including a name that cannot be unqualified",
		config: "rules:\n  demote: true\n",
		files: map[string]string{
			"only.go": "package x\n\nfunc onlyHelper() int { return 1 }\n\nfunc onlyType() int { return 2 }\n\nvar onlyID = 3\n\nfunc Exported() int { return onlyHelper() + onlyType() + onlyID }\n",
		},
	},
	{
		name:   "promote required in a single-namespace package",
		config: "rules:\n  promote: true\n",
		files: map[string]string{
			"only.go": "package x\n\nfunc helper() int { return 1 }\n\ntype shape struct{ side int }\n\nfunc Exported() int { return helper() + shape{}.side }\n",
		},
	},
	{
		name: "type embedded, and selected through the embedding, in its own namespace",
		files: map[string]string{
			"user.go":  "package x\n\ntype count struct{ n int }\n\ntype User struct{ *count }\n\nfunc (u *User) Total() int { return u.count.n }\n",
			"order.go": "package x\n\nfunc OrderRun() {}\n",
		},
	},
	{
		name: "type embedded from another namespace",
		files: map[string]string{
			"user.go":  "package x\n\ntype userCount struct{ n int }\n",
			"order.go": "package x\n\ntype Order struct{ userCount }\n",
		},
	},
	{
		name: "scope stated by a directive, which must not be rewritten",
		files: map[string]string{
			"user.go":  "package x\n\n//declscope:file\nfunc userForced() int { return 1 }\n",
			"order.go": "package x\n\nfunc orderRun() int { return userForced() }\n\nvar _ = orderRun\n",
		},
	},
}

func TestSuggestedFixesConverge(t *testing.T) {
	bin := buildBinary(t)
	for _, tc := range fixCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "go.mod", "module x\n\ngo 1.25\n")
			if tc.config != "" {
				write(t, dir, ".declscope.yaml", tc.config)
			}
			for name, body := range tc.files {
				write(t, dir, name, body)
			}
			mustCompile(t, dir, "the case's own input")

			before := diagnose(t, bin, dir)

			// Exactly one pass. Running -fix repeatedly would hide a fix that
			// only works the second time around, and nobody runs it twice.
			run(t, bin, dir, "-fix", "./...")

			mustCompile(t, dir, "the result of -fix")
			after := diagnose(t, bin, dir)

			for _, d := range before {
				if d.fixed && after.has(d) {
					t.Errorf("applying its fix did not remove the diagnostic:\n  %s\n  %s", d.category, d.message)
				}
			}
			for _, d := range after {
				if !before.has(d) {
					t.Errorf("-fix introduced a diagnostic that was not there before:\n  %s\n  %s", d.category, d.message)
				}
			}
			if t.Failed() {
				for name := range tc.files {
					body, _ := os.ReadFile(filepath.Join(dir, name))
					t.Logf("--- %s after -fix ---\n%s", name, body)
				}
			}
		})
	}
}

type diagnostic struct {
	category string
	message  string
	fixed    bool
}

type diagnostics []diagnostic

func (ds diagnostics) has(want diagnostic) bool {
	for _, d := range ds {
		if d.category == want.category && d.message == want.message {
			return true
		}
	}
	return false
}

func diagnose(t *testing.T, bin, dir string) diagnostics {
	t.Helper()
	out := run(t, bin, dir, "-json", "./...")

	// map[package]map[analyzer][]diagnostic
	var raw map[string]map[string][]struct {
		Category string `json:"category"`
		Message  string `json:"message"`
		Fixes    []struct {
			Message string `json:"message"`
		} `json:"suggested_fixes"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("parsing -json output: %v\n%s", err, out)
	}
	var ds diagnostics
	for _, byAnalyzer := range raw {
		for _, list := range byAnalyzer {
			for _, d := range list {
				ds = append(ds, diagnostic{category: d.Category, message: d.Message, fixed: len(d.Fixes) > 0})
			}
		}
	}
	return ds
}

// run executes the linter and returns its stdout. A non-zero exit is expected
// whenever there are diagnostics, so only a failure to start is fatal.
func run(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !asExitError(err, &exit) {
			t.Fatalf("running declscope %v: %v\n%s", args, err, stderr.String())
		}
	}
	return stdout.String()
}

func asExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

func mustCompile(t *testing.T, dir, what string) {
	t.Helper()
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s does not compile: %v\n%s", what, err, out)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "declscope")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/declscope")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the linter: %v\n%s", err, out)
	}
	return bin
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
