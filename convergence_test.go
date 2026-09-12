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
// A golden file pins what a fix produces; only applying it and looking again
// pins that it helped. A fix can match its golden exactly and still violate
// (2) or (3): a directive emitted next to a conflicting directive produces
// code the linter itself rejects, and a directive inserted on the wrong line
// for a field of a single-line struct binds to the struct instead of the
// field.

type fixCase struct {
	name   string
	config string
	files  map[string]string
}

var fixCases = []fixCase{
	{
		name: "boundary and qualify on the same declaration",
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
		name:   "unqualify, including a name that cannot be unqualified",
		config: "rules:\n  unqualify: always\n",
		files: map[string]string{
			"only.go": "package x\n\nfunc onlyHelper() int { return 1 }\n\nfunc onlyType() int { return 2 }\n\nvar onlyID = 3\n\nfunc Exported() int { return onlyHelper() + onlyType() + onlyID }\n",
		},
	},
	{
		name:   "qualify required in a single-namespace package",
		config: "rules:\n  qualify: always\n",
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
			"user.go":  "package x\n\n//declscope:private\nfunc userForced() int { return 1 }\n",
			"order.go": "package x\n\nfunc orderRun() int { return userForced() }\n\nvar _ = orderRun\n",
		},
	},

	// A rename is offered only when it provably changes nothing but the
	// spelling. Each case below is one way a rename that checks only package
	// scope would either fail to compile or, worse, compile into a program
	// computing something else. The cases are arranged so that the wrong
	// rename fails to compile, since this test cannot run the result: the
	// parameter capture, for instance, makes the captured reference the wrong
	// type.
	{
		name:   "rename captured by a parameter or a local at a reference",
		config: "rules:\n  qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nvar count = 10\n\n" +
				"func Add(fooCount string) int { return len(fooCount) + count }\n\n" +
				"func Sub() int {\n\tfooCount := \"x\"\n\treturn len(fooCount) - count\n}\n",
		},
	},
	{
		name:   "rename colliding with an import in another file",
		config: "rules:\n  qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nvar count = 10\n\nfunc Add(x int) int { return x + count }\n",
			"bar.go": "package x\n\nimport fooCount \"strings\"\n\nfunc Bar(s string) string { return fooCount.ToUpper(s) }\n",
		},
	},
	{
		name:   "rename to a predeclared name",
		config: "rules:\n  unqualify: always\n",
		files: map[string]string{
			"only.go": "package x\n\nvar onlyLen = 3\n\nfunc Exported(s string) int { return len(s) + onlyLen }\n",
		},
	},
	{
		name:   "two unqualify renames converging on one name",
		config: "rules:\n  qualify: never\n  unqualify: always\n",
		files: map[string]string{
			"a.go": "package x\n\nvar aFoo = 1\n\nvar _ = aFoo\n",
			"b.go": "package x\n\nvar bFoo = 2\n\nvar _ = bFoo\n",
		},
	},
	{
		name:   "two unqualify renames converging through initialism lowering",
		config: "rules:\n  unqualify: always\n",
		files: map[string]string{
			"only.go": "package x\n\nvar onlyBar = 1\n\nvar onlyBAR = 2\n\nvar _, _ = onlyBar, onlyBAR\n",
		},
	},
	{
		name: "two qualify renames converging through a non-injective label",
		files: map[string]string{
			"a.go":   "package x\n\nfunc bX() int { return 1 }\n\nvar _ = bX\n",
			"a_b.go": "package x\n\nfunc x() int { return 2 }\n\nvar _ = x\n",
		},
	},
	{
		name: "reference in a generated file",
		files: map[string]string{
			"user.go":   "package x\n\n//declscope:package\nfunc helper() int { return 1 }\n",
			"order.go":  "package x\n\nfunc OrderRun() int { return helper() }\n",
			"zz_gen.go": "// Code generated by a tool. DO NOT EDIT.\n\npackage x\n\nfunc GenUse() int { return helper() }\n",
		},
	},
	{
		name:   "reference in an excluded file",
		config: "exclude:\n  - \"**/ext.go\"\n",
		files: map[string]string{
			"user.go":  "package x\n\n//declscope:package\nfunc helper() int { return 1 }\n",
			"order.go": "package x\n\nfunc OrderRun() int { return helper() }\n",
			"ext.go":   "package x\n\nfunc ExtUse() int { return helper() }\n",
		},
	},
	{
		name:   "declaration named by a go:linkname directive",
		config: "rules:\n  qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nimport _ \"unsafe\"\n\n//go:linkname helper\nfunc helper() int { return 1 }\n\nfunc Exported() int { return helper() }\n",
		},
	},
	{
		name:   "rename target declared only in the test variant",
		config: "rules:\n  qualify: always\n",
		files: map[string]string{
			"foo.go":      "package x\n\nvar count = 10\n\nfunc Add(x int) int { return x + count }\n",
			"foo_test.go": "package x\n\nvar fooCount = 1\n\nvar _ = fooCount\n",
		},
	},
	{
		// The positive counterpart: with tests present the rename must still
		// happen, and reach the test file, through the variant that sees it.
		name:   "rename referenced from a test file",
		config: "rules:\n  qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nvar count = 10\n\nfunc Add(x int) int { return x + count }\n",
			"foo_test.go": "package x\n\nimport \"testing\"\n\n" +
				"func TestAdd(t *testing.T) {\n\tif Add(1) != 1+count {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n",
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

// mustCompile type-checks the package through go vet rather than go build,
// because vet also type-checks the test variant: a rename applied to the
// non-test files alone builds and then fails the first go test.
func mustCompile(t *testing.T, dir, what string) {
	t.Helper()
	cmd := exec.Command("go", "vet", "./...")
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
