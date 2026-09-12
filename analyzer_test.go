package declscope_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/declscope"
)

func TestPackageLevel(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "pkglevel")
}

func TestMembers(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "members")
}

func TestDirectives(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "directives")
}

// TestTestFilesShareNamespace checks that a _test.go file may use the
// file-private declarations of its subject.
func TestTestFilesShareNamespace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "testns")
}

// TestGeneratedFilesExcluded checks that generated files are neither checked
// nor treated as reference sites.
func TestGeneratedFilesExcluded(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "generated")
}

// TestDemotion checks the opt-in rule that reports a namespace prefix claiming
// more reach than it uses. It is enabled by testdata/src/demotion/.declscope.yaml.
func TestDemotion(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "demotion")
}

// TestForeignMethods checks the opt-in rule for unexported methods grown on a
// type belonging to another namespace.
func TestForeignMethods(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "foreign")
}

// TestNamespaceDirectiveDetached checks that a namespace directive separated
// from the package clause by a blank line still applies to the file.
func TestNamespaceDirectiveDetached(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "nsdetached")
}

// TestNamespaceDirectiveInPackageDoc checks the placement prescribed by Go's
// doc comment convention: a directive at the bottom of the package comment,
// which go/doc strips from the rendered documentation.
func TestNamespaceDirectiveInPackageDoc(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "nsindoc")
}

// TestBaseline checks that recorded violations are suppressed while new ones
// are still reported. The baseline lives in testdata/src/baselined and is
// found by the same upward lookup as the config file.
func TestBaseline(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "baselined")
}

// TestExplicitScopeConflict checks that a declaration whose scope was stated
// by a directive is reported without a fix, since -fix must not overwrite a
// decision the author made deliberately.
func TestExplicitScopeConflict(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "explicit")
}
