//declscope:namespace analyzer

package declscope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// TestOptionsErrorCarriesNoAnalyzerPrefix checks that a config error is
// returned bare. The driver prints every analyzer error as "<name>: <err>",
// so a prefix added here would come out as "declscope: declscope: ...".
func TestOptionsErrorCarriesNoAnalyzerPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".declscope.yaml")
	if err := os.WriteFile(path, []byte("rules: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newAnalyzer()
	if err := a.Flags.Set("config", path); err != nil {
		t.Fatal(err)
	}

	_, err := options(&analysis.Pass{Analyzer: a})
	if err == nil {
		t.Fatal("want an error for a malformed config, got none")
	}
	if strings.HasPrefix(err.Error(), a.Name+":") {
		t.Errorf("error carries the analyzer's name, which the driver adds again: %q", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error does not name the config file: %q", err)
	}
}
