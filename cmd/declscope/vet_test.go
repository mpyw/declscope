package main_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestVetAgreesOnNonGoFiles checks that `go vet -vettool` and a direct run
// withhold the same rename and the same surplus report beside a non-Go file
// (#285). go/packages lists .h and .syso files in OtherFiles, but the go
// command hands vet only its .s and .c files, and none of them in a cgo
// package. The analyzer reads the fact from the folder instead, so the two
// drivers see the same files.
func TestVetAgreesOnNonGoFiles(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	for _, other := range []string{"x.syso", "x.h"} {
		t.Run(other, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, "go.mod", testModule)
			writeTree(t, dir, ".declscope.yaml", "rules:\n  naming:\n    qualify: always\n")
			// The other file may name add, so it is not renamed.
			writeTree(t, dir, "p/user.go", "package p\n\nfunc add(a, b int64) int64 { return a + b }\n\nfunc Sum() int64 { return add(1, 2) }\n")
			writeTree(t, dir, "p/"+other, "")
			// The other file may be the use, so the directive is not surplus.
			writeTree(t, dir, "q/user.go", "package q\n\n//declscope:shared\nfunc userHelper() {}\n\nfunc UserRun() { userHelper() }\n")
			writeTree(t, dir, "q/"+other, "")

			direct, _ := runIn(t, bin, dir, "-json", "./p", "./q")
			vet, _ := runIn(t, "go", dir, "vet", "-vettool="+bin, "-json", "./p", "./q")
			for name, out := range map[string]string{"direct": direct, "go vet": vet} {
				if !strings.Contains(out, "func add does not carry") {
					t.Fatalf("%s: the finding the fixture depends on is missing:\n%s", name, out)
				}
				if strings.Contains(out, "rename add to userAdd") {
					t.Errorf("%s: offered the rename beside %s:\n%s", name, other, out)
				}
				if strings.Contains(out, "no use from another namespace") {
					t.Errorf("%s: reported surplus beside %s:\n%s", name, other, out)
				}
			}
		})
	}
}
