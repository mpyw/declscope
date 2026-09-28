package main_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLineDirectiveIgnore runs the linter over a file-level ignore above a
// //line directive. The directive renames the positions below it, but they are
// still in the file the ignore covers, so nothing is reported. The control is
// the same file without the directive.
func TestLineDirectiveIgnore(t *testing.T) {
	for name, line := range map[string]string{"with //line": "//line x.tmpl:1\n", "without": ""} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, "go.mod", testModule)
			writeTree(t, dir, "p/x.go", "//declscope:ignore\n\npackage p\n\n"+line+"//declscope:bogus\nfunc run() {}\n")
			if out, code := runIn(t, bin, dir, "./..."); code != 0 || out != "" {
				t.Errorf("exit %d, want 0 and no output:\n%s", code, out)
			}
		})
	}
}

// TestLineDirectiveFix runs -fix over a field in the middle of a line, below a
// //line directive without a column. The adjusted column is 0 there. The fix
// still breaks the line before the field, so the directive binds to it, and a
// second run reports nothing.
func TestLineDirectiveFix(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "go.mod", testModule)
	writeTree(t, dir, "p/order.go", "package p\n\nfunc orderRead() bool { return UserMake().flag }\n\nvar _ = orderRead\n")
	writeTree(t, dir, "p/user.go", "package p\n\n//line user.tmpl:1\ntype kept struct{ flag bool }\n\nfunc UserMake() kept { return kept{flag: true} }\n")

	runIn(t, bin, dir, "-fix", "./...")
	got, err := os.ReadFile(filepath.Join(dir, "p/user.go"))
	if err != nil {
		t.Fatal(err)
	}
	want := "package p\n\n//line user.tmpl:1\ntype kept struct {\n\t//declscope:package\n\tflag bool\n}\n\nfunc UserMake() kept { return kept{flag: true} }\n"
	if string(got) != want {
		t.Errorf("-fix wrote:\n%s\nwant:\n%s", got, want)
	}
	if out, code := runIn(t, bin, dir, "./..."); code != 0 || out != "" {
		t.Errorf("after -fix: exit %d, want 0 and no output:\n%s", code, out)
	}
}
