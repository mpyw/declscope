package declscope_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/module"
)

// TestModuleZipPaths checks every file the module zip would carry against the
// module path rules. One file the rules refuse makes the whole version
// uninstallable with go install, go run and go get -tool, while the test suite
// and a prebuilt binary still work.
func TestModuleZipPaths(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == ".git" {
				return filepath.SkipDir
			}
			// A nested module is left out of the zip.
			if path != "." {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if err := module.CheckFilePath(filepath.ToSlash(path)); err != nil {
			t.Error(err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
