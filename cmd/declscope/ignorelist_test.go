package main_test

import (
	"strings"
	"testing"
)

// TestIgnoreEmptyRuleName runs the linter over an ignore whose rule list holds
// an empty name. It used to be skipped, so //declscope:ignore , silenced every
// rule. It is reported now, and silences nothing.
func TestIgnoreEmptyRuleName(t *testing.T) {
	for name, directive := range map[string]string{"comma": "//declscope:ignore ,", "trailing comma": "//declscope:ignore boundary,"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, "go.mod", testModule)
			writeTree(t, dir, "p/user.go", "package p\n\n"+directive+"\nfunc userMake() int { return 1 }\n")
			writeTree(t, dir, "p/order.go", "package p\n\nfunc orderRun() int { return userMake() }\n\nvar _ = orderRun\n")
			out, code := runIn(t, bin, dir, "./...")
			if code == 0 {
				t.Fatalf("exit 0, want reports:\n%s", out)
			}
			for _, want := range []string{
				"user.go:3:1: empty rule name in declscope:ignore",
				`func userMake is private to namespace "user", but is used from namespace "order"`,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%s", want, out)
				}
			}
		})
	}
}
