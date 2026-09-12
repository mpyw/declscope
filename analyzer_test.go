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

// TestPromote checks that an unexported package-level declaration must carry
// its namespace as a label, and that members and exported identifiers are
// exempt.
func TestPromote(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "promoterule")
}

// TestSingleNamespace checks that the label is not required in a package with
// only one namespace, where there is no boundary for it to mark.
func TestSingleNamespace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "singlens")
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

// TestPromoteAlways checks rules.promote: true, which requires the label even
// in a package with a single namespace.
func TestPromoteAlways(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "promotealways")
}

// TestDemote checks the mirror of the label rule: where the label is not
// required, it must not be present.
func TestDemote(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "demote")
}

// TestDemoteInert checks that demote says nothing wherever the label is
// required, so the two rules can never contradict each other.
func TestDemoteInert(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "demoteinert")
}

// TestIgnoreScope checks that an ignore directive can name the rules it
// silences, and that each directive is reported unused on its own.
func TestIgnoreScope(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "ignorescope")
}

// TestFileIgnore checks that an ignore directive before the package clause
// applies to every declaration in the file, and is reported when it silences
// nothing.
func TestFileIgnore(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "fileignore")
}

// TestFileIgnoreIsFileScoped checks that a file-level ignore covers only the
// file carrying it, not every file sharing its namespace.
func TestFileIgnoreIsFileScoped(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "fileignorescope")
}

// TestTypeIgnore checks that a member inherits the ignore directives written
// on the type that owns it, wherever the member is declared, and that a
// directive used up only by a member still counts as used.
func TestTypeIgnore(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "typeignore")
}

// TestDefaultsReachMembers checks that defaults.exported and
// defaults.unexported resolve members too, not only package-level
// declarations.
func TestDefaultsReachMembers(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "defaultsmembers")
}
