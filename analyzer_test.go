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
// private declarations of its subject.
func TestTestFilesShareNamespace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "testns")
}

// TestGeneratedFilesExcluded checks that generated files are neither checked
// nor treated as reference sites.
func TestGeneratedFilesExcluded(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "generated")
}

// TestQualify checks that an unexported package-level declaration must carry
// its namespace as a label, and that members and exported identifiers are
// exempt.
func TestQualify(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "qualifyrule")
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

// TestQualifyAlways checks rules.qualify: always, which requires the label even
// in a package with a single namespace.
func TestQualifyAlways(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "qualifyalways")
}

// TestUnqualify checks the mirror of the label rule: where the label is not
// required, it must not be present.
func TestUnqualify(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "unqualify")
}

// TestUnqualifyInert checks that unqualify says nothing wherever the label is
// required, so the two rules can never contradict each other.
func TestUnqualifyInert(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "unqualifyinert")
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

// TestDefaultsReachMembers checks that defaults.unexported resolves members
// too, not only package-level declarations.
func TestDefaultsReachMembers(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "defaultsmembers")
}

// TestGenerics checks that the members of a generic type are bounded like
// those of any other type. go/types records the instantiated field or method
// for a selection on List[int], and on List[T] inside the type's own methods,
// so the lookup has to normalize to the origin object.
func TestGenerics(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "generics")
}

// TestEmbedded checks that embedding a type is a use of it. The embedded
// field's ident is both a definition (the field) and a use (the type), and
// the second role is the one boundary has to see.
func TestEmbedded(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "embedded")
}

// TestMemberOwnerFile checks the split between a field and a method. A field is
// written inside its type's declaration, so the type's file bounds it wherever
// it is read; a method is an ordinary top-level declaration and belongs to the
// file that wrote it. Binding methods to their type's file instead left one
// unusable from the file declaring it, with no scope able to say otherwise.
func TestMemberOwnerFile(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "memberowner")
}

// TestCoreNamespace checks the core: several files share it, it has no name, and
// both naming rules pass it by at their strictest settings, while an ordinary
// namespace in the same package is still asked for its label.
func TestCoreNamespace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "corens")
}

// TestFileScope checks a file-level scope directive: a default for what the file
// declares, which a declaration may still narrow back. Under the ignore this
// replaces, the narrowing would have meant nothing.
func TestFileScope(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "filescope")
}

// TestExportedLabels checks rules.exportedLabels, and that the label carries the
// exportedness of the name it joins: New is reported against ClientNew, never
// clientNew, since a rename must not delete the API it is renaming.
func TestExportedLabels(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "exportedlabels")
}

// TestExportedScope checks that exportedness decides the default and nothing
// else: an exported declaration carries no boundary until a directive gives it
// one, and a directive on a type reaches its exported fields — which is how a
// DTO capitalized for a serializer is protected, without the analysis guessing
// at reachability it cannot compute.
func TestExportedScope(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "exportedscope")
}

// TestUnusedScopeDirective checks the structural test: a scope directive that
// binds nothing is reported, an exported type with an unexported field still
// binds one, and restating the scope already in force is not reported at all.
// The two ways a block's directive can reach nothing — every spec overriding
// it, and nothing checked being able to carry it — are reported apart.
func TestUnusedScopeDirective(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "unusedscope")
}

// TestNamespaceIdentity checks that a file whose stem cannot be a label, such
// as 2fa.go, is still a namespace: its test file shares it, a use from another
// namespace is reported, and the label rule asks nothing of it.
func TestNamespaceIdentity(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "nsidentity")
}

// TestNamespaceNormalize checks that separators other than _ and a PascalCase
// stem yield a lowerCamelCase namespace, so that the rename offered by qualify
// is a valid unexported identifier.
func TestNamespaceNormalize(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "nsnormalize")
}

// TestNamespaceInitialism checks that a namespace spells its initialisms the
// way Go does, that the label is matched ignoring case, and that the rename
// offered by qualify spells the name's first word the same way.
func TestNamespaceInitialism(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "nsinitialism")
}

// TestBlockIgnore checks that an ignore directive is judged once per comment,
// not once per declaration it reaches: one on a block, or shared by the names
// of one spec, is used as soon as any of them needed it, and is reported once
// when none did.
func TestBlockIgnore(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "blockignore")
}

// TestTestOnlyIgnore checks that a directive needed only by a reference in a
// _test.go file is not reported unused by the ordinary variant of the package,
// which cannot see that reference. Only the test variant, which sees every
// file, judges, and a directive in a test file shows that it does.
func TestTestOnlyIgnore(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "testonlyignore")
}

// TestBraceIgnore checks the two placements go/parser attaches to nothing: a
// trailing comment on the first or last line of a multi-line declaration binds
// to it, and a directive anywhere else after the package clause is reported as
// misplaced rather than silently dropped.
func TestBraceIgnore(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "braceignore")
}

// TestAliases checks what a directive on a type alias reaches. The alias name
// is an ordinary declaration, so //declscope:package on it widens the name and
// use.go may take an al — the silence on alias.go is the assertion. It does not
// reach the aliased defined type's members, so Base.n keeps base.go's boundary.
// An alias to a struct written inline does contain its fields.
func TestAliases(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "aliases")
}
