package pattern

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestUnmatchedLoadsOnlyWhenAsked pins the two answers known without a load:
// nothing loaded means every pattern matched nothing, and one pattern with a
// package loaded means it matched. A driver that fails every load proves that
// neither case loads anything.
func TestUnmatchedLoadsOnlyWhenAsked(t *testing.T) {
	driver := filepath.Join(t.TempDir(), "driver")
	if err := os.WriteFile(driver, []byte("#!/bin/sh\necho driver refused >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPACKAGESDRIVER", driver)
	cfg := &packages.Config{Dir: t.TempDir()}
	one := []*packages.Package{{PkgPath: "example.com/p"}}

	if _, err := Unmatched(cfg, []string{"./a/...", "./b/..."}, nil); err == nil || err.Error() != "./a/... ./b/... matched no packages" {
		t.Errorf("err = %v, want every pattern named in the go command's words", err)
	}
	if got, err := Unmatched(cfg, []string{"./p"}, one); err != nil || got != nil {
		t.Errorf("Unmatched = %v, %v, want nothing unmatched and no load", got, err)
	}
	// With two patterns, each is loaded on its own, and a failed load is an
	// error rather than a pattern read as unmatched.
	if _, err := Unmatched(cfg, []string{"./a/...", "./p"}, one); err == nil || !strings.Contains(err.Error(), "driver refused") {
		t.Errorf("err = %v, want the driver's failure", err)
	}
}
