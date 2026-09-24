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

// strictConfig puts surplus's per-declaration judgment in force, which the
// default leaves off.
const strictConfig = "rules:\n  surplus: strict\n"

// directiveStrictConfig puts the directive rule's strict judgment in force,
// whose fix deletes a directive that restates the scope in force.
const directiveStrictConfig = "rules:\n  directive: strict\n"

type fixCase struct {
	name   string
	config string
	files  map[string]string
}

var fixCases = []fixCase{
	{
		name:   "boundary and qualify on the same declaration",
		config: "rules:\n  naming:\n    qualify: ondemand\n",
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
		name:   "grouped var and const blocks",
		config: "rules:\n  naming:\n    qualify: ondemand\n",
		files: map[string]string{
			"user.go":  "package x\n\nvar (\n\tseed  = 1\n\tother = 2\n)\n\nconst (\n\tlimit = 3\n)\n",
			"order.go": "package x\n\nfunc orderRun() int { return seed + other + limit }\n\nvar _ = orderRun\n",
		},
	},
	{
		// Two directive insertions converging: the one on the type reaches the
		// field, so a second one on the field would bind nothing, and the
		// directive rule would report what -fix had just written.
		name: "type and member crossing together",
		files: map[string]string{
			"user.go":  "package x\n\ntype entry struct {\n\tkey string\n}\n",
			"order.go": "package x\n\nfunc Order() string {\n\te := entry{key: \"x\"}\n\treturn e.key\n}\n",
		},
	},
	{
		name:   "rename target already taken",
		config: "rules:\n  naming:\n    qualify: ondemand\n",
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
		name:   "qualify required in a single-namespace package",
		config: "rules:\n  naming:\n    qualify: always\n",
		files: map[string]string{
			"only.go": "package x\n\nfunc helper() int { return 1 }\n\ntype shape struct{ side int }\n\nfunc Exported() int { return helper() + shape{}.side }\n",
		},
	},
	{
		name:   "type embedded, and selected through the embedding, in its own namespace",
		config: "rules:\n  naming:\n    qualify: ondemand\n",
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

	// strict's fix narrows a declaration with //declscope:private. It must
	// bind, and it must not leave the enclosing directive binding nothing.
	{
		name:   "fields a type's directive widens for nothing, under strict",
		config: strictConfig,
		files: map[string]string{
			"user.go": "package x\n\n//declscope:package\ntype account struct {\n\tid int\n\n" +
				"\t// balance is local.\n\tbalance int\n\tleft, right int\n}\n\n" +
				"//declscope:package\ntype tiny struct{ id int; n int }\n\n" +
				"func userLocal(a account, t tiny) int { return a.balance + a.left + a.right + t.n }\n\nvar _ = userLocal\n",
			"order.go": "package x\n\nfunc orderRun(a account, t tiny) int { return a.id + t.id }\n\nvar _ = orderRun\n",
		},
	},
	{
		// A type under the file's directive is narrowed with its fields, and a
		// type one of whose fields is read outside keeps its scope while its
		// other field is narrowed alone.
		name:   "declarations a file's directive widens for nothing, under strict",
		config: strictConfig,
		files: map[string]string{
			"user.go": "//declscope:package\n\npackage x\n\n// userShared is called from order.go.\nfunc userShared() int { return userLocal() }\n\n" +
				"// userLocal is not.\nfunc userLocal() int { return 1 }\n\nvar userA, userB = 1, 2\n\n" +
				"type box struct {\n\tn int\n}\n\ntype pair struct {\n\tx int\n\ty int\n}\n\n" +
				"func userPair() pair { return pair{} }\n\nvar _ = userA + userB + box{}.n + userPair().y\n",
			"order.go": "package x\n\nfunc orderRun() int { return userShared() + userPair().x }\n\nvar _ = orderRun\n",
		},
	},
	{
		// Narrowing the field would leave the directive deciding nothing,
		// which the directive rule would report, so it gets no fix.
		name:   "a field whose narrowing would leave the directive unused, under strict",
		config: strictConfig,
		files: map[string]string{
			"user.go": "package x\n\n//declscope:package\ntype DTO struct {\n\tName string\n\tseq  int\n}\n\n" +
				"func userSeq(d DTO) int { return d.seq }\n\nvar _ = userSeq\n",
		},
	},
	{
		// A boundary fix on a type widens its fields as well, and under strict
		// the ones no other namespace reads would then be reported. The same
		// fix narrows them, in both variants of a package with a test file.
		name:   "boundary fix on a type whose fields nobody else reads, under strict",
		config: strictConfig,
		files: map[string]string{
			"user.go": "package x\n\ntype item struct {\n\tshared int\n\n\t// count is local.\n\tcount int\n}\n\n" +
				"func userCount(i item) int { return i.count }\n\nvar _ = userCount\n",
			"order.go":     "package x\n\nfunc orderRun(i item) int { return i.shared }\n\nvar _ = orderRun\n",
			"user_test.go": "package x\n\nimport \"testing\"\n\nfunc TestItem(t *testing.T) {\n\tif userCount(item{count: 1}) != 1 {\n\t\tt.Fatal(\"no\")\n\t}\n}\n",
		},
	},

	// rules.directive: strict deletes a directive naming the scope its
	// declarations would have without it. Deleting several in one run must
	// leave every scope where it was, and change no other report.
	{
		name:   "redundant directives at every level, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go": "//declscope:private\n\npackage x\n\n// userDoc is documented.\n//\n//declscope:private\nfunc userDoc() int { return 1 }\n\n" +
				"//declscope:private\ntype userShape struct {\n\t//declscope:private\n\tside int\n\tedge int //declscope:private\n}\n\n" +
				"//declscope:private\nvar (\n\t//declscope:private\n\tuserSeed = 1\n\tuserLimit = 2\n)\n\n" +
				"var _ = userDoc() + userShape{}.side + userShape{}.edge + userSeed + userLimit\n",
			"order.go": "package x\n\nfunc OrderRun() int { return 1 }\n",
		},
	},
	{
		// The spec's private narrows its block's package, so it is kept, and
		// the file's private, which the spec's shadows, is judged against it.
		name:   "a spec narrowing its block, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go": "//declscope:private\n\npackage x\n\n//declscope:package\nvar (\n\t//declscope:private\n\tuserA = 1\n)\n\nvar _ = userA\n",
		},
	},
	{
		// A redundant directive on a declaration used elsewhere keeps it: the
		// boundary report names the level that decided.
		name:   "a redundant directive on a crossing declaration, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go":  "package x\n\n//declscope:private\nfunc userShared() int { return 1 }\n\n//declscope:private\ntype userBox struct {\n\tn int\n}\n",
			"order.go": "package x\n\nfunc OrderRun() int { return userShared() + userBox{}.n }\n",
		},
	},
	{
		// The boundary fix widens the type, so the field would take the new
		// directive if its own were deleted in the same run, and surplus
		// strict would report it widened for nothing.
		name:   "a field's redundant directive under a type the boundary fix widens, under directive strict",
		config: "rules:\n  directive: strict\n  surplus: strict\n",
		files: map[string]string{
			"user.go":  "package x\n\ntype entry struct {\n\t//declscope:private\n\tkey string\n}\n\nfunc userKey(e entry) string { return e.key }\n\nvar _ = userKey\n",
			"order.go": "package x\n\nfunc OrderRun() entry { return entry{} }\n",
		},
	},
	{
		// Every spec overrides the block, whose report an ignore answers.
		// userA's directive restates the block and is deleted. The block then
		// binds userA, but is still redundant and still answered.
		name:   "a block every spec overrides, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go": "package x\n\n//declscope:private\n//declscope:ignore directive\nvar (\n\t//declscope:private\n\tuserA = 1\n\t//declscope:package\n\tUserB = 2\n)\n\nvar _ = userA\n",
		},
	},
	{
		// The block narrows UserA, so it is not redundant, and a bare ignore
		// answers its loose report. Deleting UserA's directive would make the
		// block bind, and leave the ignore answering nothing.
		name:   "a spec restating a block a bare ignore answers, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go": "package x\n\n//declscope:private\n//declscope:ignore\nvar (\n\t//declscope:private\n\tUserA = 1\n)\n",
		},
	},
	{
		// Under the package default surplus reports the same directive; the
		// deletion settles both. Where an ignore answers surplus, the fix is
		// withheld, or the ignore would be left answering nothing.
		name:   "a package directive restating the package default, under directive strict",
		config: "defaults:\n  unexported: package\nrules:\n  directive: strict\n",
		files: map[string]string{
			"user.go": "package x\n\n//declscope:package\nfunc userHelper() int { return 1 }\n\n" +
				"//declscope:package\n//declscope:ignore surplus\nfunc userQuiet() int { return 2 }\n\nvar _ = userHelper() + userQuiet()\n",
		},
	},
	{
		// Deleting the member's directive would make it a dependent of the
		// file's, which surplus strict judges one declaration at a time.
		name:   "a package directive restating the file's, under directive and surplus strict",
		config: "rules:\n  directive: strict\n  surplus: strict\n",
		files: map[string]string{
			"user.go":  "//declscope:package\n\npackage x\n\nfunc userShared() int { return 1 }\n\n//declscope:package\nfunc userLocal() int { return 2 }\n\nvar _ = userLocal()\n",
			"order.go": "package x\n\nfunc OrderRun() int { return userShared() }\n",
		},
	},
	{
		// Only the test variant sees the crossing. The ordinary variant must
		// not offer the deletion the test variant withholds, since the driver
		// applies the fixes of both.
		name:   "a redundant directive on a declaration only a test uses from elsewhere, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go":       "package x\n\n//declscope:private\nfunc userShared() int { return 1 }\n",
			"order_test.go": "package x\n\nimport \"testing\"\n\nfunc TestOrder(t *testing.T) { _ = userShared() }\n",
		},
	},
	{
		// The block keeps its report, since another namespace uses userA.
		// Deleting userB's directive would hand userB to the block, and the
		// block's report would name it too.
		name:   "a spec restating a block that keeps its report, under directive strict",
		config: directiveStrictConfig,
		files: map[string]string{
			"user.go":  "package x\n\n//declscope:private\nvar (\n\tuserA = 1\n\t//declscope:private\n\tuserB = 2\n)\n\nvar _ = userB\n",
			"order.go": "package x\n\nvar _ = userA\n",
		},
	},
	{
		// The block binds nothing, but narrowed needs it, so it is reported
		// by loose alone. Deleting Exported's directive would hand Exported
		// to the block, and the block's report would name it.
		name:   "a spec restating a block that is reported but not redundant, under directive strict",
		config: "rules:\n  directive: strict\n  surplus: off\n",
		files: map[string]string{
			"user.go": "package x\n\n//declscope:package\nvar (\n\t//declscope:package\n\tExported = 1\n\t//declscope:private\n\tnarrowed = 2\n)\n\nvar _ = narrowed\n",
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
		config: "rules:\n  naming:\n    qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nvar count = 10\n\n" +
				"func Add(fooCount string) int { return len(fooCount) + count }\n\n" +
				"func Sub() int {\n\tfooCount := \"x\"\n\treturn len(fooCount) - count\n}\n",
		},
	},
	{
		name:   "rename colliding with an import in another file",
		config: "rules:\n  naming:\n    qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nvar count = 10\n\nfunc Add(x int) int { return x + count }\n",
			"bar.go": "package x\n\nimport fooCount \"strings\"\n\nfunc Bar(s string) string { return fooCount.ToUpper(s) }\n",
		},
	},
	{
		name:   "two qualify renames converging through a non-injective prefix",
		config: "rules:\n  naming:\n    qualify: ondemand\n",
		files: map[string]string{
			"a.go":   "package x\n\nfunc bX() int { return 1 }\n\nvar _ = bX\n",
			"a_b.go": "package x\n\nfunc x() int { return 2 }\n\nvar _ = x\n",
		},
	},
	{
		name:   "reference in a generated file",
		config: "rules:\n  naming:\n    qualify: ondemand\n",
		files: map[string]string{
			"user.go":   "package x\n\n//declscope:package\nfunc helper() int { return 1 }\n",
			"order.go":  "package x\n\nfunc OrderRun() int { return helper() }\n",
			"zz_gen.go": "// Code generated by a tool. DO NOT EDIT.\n\npackage x\n\nfunc GenUse() int { return helper() }\n",
		},
	},
	{
		name:   "reference in an omitted file",
		config: "rules:\n  naming:\n    qualify: ondemand\nfilter:\n  omit:\n    - \"**/ext.go\"\n",
		files: map[string]string{
			"user.go":  "package x\n\n//declscope:package\nfunc helper() int { return 1 }\n",
			"order.go": "package x\n\nfunc OrderRun() int { return helper() }\n",
			"ext.go":   "package x\n\nfunc ExtUse() int { return helper() }\n",
		},
	},
	{
		name:   "declaration named by a go:linkname directive",
		config: "rules:\n  naming:\n    qualify: always\n",
		files: map[string]string{
			"foo.go": "package x\n\nimport _ \"unsafe\"\n\n//go:linkname helper\nfunc helper() int { return 1 }\n\nfunc Exported() int { return helper() }\n",
		},
	},
	{
		name:   "rename target declared only in the test variant",
		config: "rules:\n  naming:\n    qualify: always\n",
		files: map[string]string{
			"foo.go":      "package x\n\nvar count = 10\n\nfunc Add(x int) int { return x + count }\n",
			"foo_test.go": "package x\n\nvar fooCount = 1\n\nvar _ = fooCount\n",
		},
	},
	{
		// The positive counterpart: with tests present the rename must still
		// happen, and reach the test file, through the variant that sees it.
		name:   "rename referenced from a test file",
		config: "rules:\n  naming:\n    qualify: always\n",
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
