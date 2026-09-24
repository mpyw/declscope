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

// TestForeignMethod checks which methods the naming rule reaches. A method
// filed away from its receiver's type is read through a receiver that names a
// different unit, so the rule asks for the namespace the method is written in.
// A local method and an interface method name are left alone.
func TestForeignMethod(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "foreignmethod")
}

// TestBoundaryOff checks that rules.boundary: off leaves the naming rule
// running on its own. A reach that would be reported is not, and a name that
// does not carry its namespace still is.
func TestBoundaryOff(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "boundaryoff")
}

// TestFilterCancelled checks the one filter report. A config beside a package
// states an only, the files it matches are all removed by an only above it,
// and the package is read as empty. Nobody writes a filter for a subtree they
// meant to exclude, so the tool says the config cannot take effect.
func TestFilterCancelled(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "filtercancelled/sub")
}

// TestEmptyFile checks that a file holding only a package clause and comments
// does not add to the namespace count. A doc.go is the usual one.
func TestEmptyFile(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "emptyfile")
}

// TestBlankImport checks the other side: a file whose only content is an import
// declares something, because the import runs.
func TestBlankImport(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "blankimport")
}

// TestIgnoreDirective checks which ignore answers a report about a directive.
// Such a report hangs on a comment rather than on a declaration, so the file
// level answers one written where no declaration is, and an ignore naming the
// rule beside another answers for its neighbour.
func TestIgnoreDirective(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "ignoredirective")
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
// its namespace as a prefix, and that members and exported identifiers are
// exempt.
func TestQualify(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "qualifyrule")
}

// TestSingleNamespace checks that the prefix is not required in a package with
// only one namespace, where there is no boundary for it to mark.
func TestSingleNamespace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "singlens")
}

// TestQualifyInflections checks the two inflections whose spelling leaves the
// namespace behind: a final e dropped before -ing (store → storing) and a
// final y turned to i (apply → applied). Only forms generated from the
// namespace are accepted, so a name that merely shares the stem (story) is
// still reported.
func TestQualifyInflections(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "qualifyinflect")
}

// TestQualifyVocabulary checks rules.naming.vocabulary: a word listed for a
// namespace satisfies the naming rule under the same test as the namespace
// itself — word boundary on the left, free right edge — and nothing more.
func TestQualifyVocabulary(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "vocabulary")
}

// TestQualifyDefaultOff checks the built-in default: with no config, the
// naming rule asks nothing even of a package with a second namespace, while
// the boundary rule fires as it always did.
func TestQualifyDefaultOff(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "qualifydefault")
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

// TestQualifyAlways checks rules.qualify: always, which requires the prefix even
// in a package with a single namespace.
func TestQualifyAlways(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "qualifyalways")
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

// TestCoreNamespace checks the core: several files share the one unnamed
// namespace, the naming rules are outside it, and an ordinary namespace in the
// same package is asked for its prefix as usual.
//
// It also checks that //declscope:core carries no scope. A core declaration is
// private to the core by default, so naming it from outside crosses a boundary,
// and a file wanting both states both.
func TestCoreNamespace(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "corens")
}

// TestFileScope checks a file-level scope directive: a default for what the file
// declares, which a declaration may still narrow back. Under the ignore this
// replaces, the narrowing would have meant nothing.
func TestFileScope(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "filescope")
}

// TestNameExported checks rules.naming.exported, and that the prefix carries the
// exportedness of the name it joins: New is reported against ClientNew, never
// clientNew, since a rename must not delete the API it is renaming.
func TestNameExported(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "exportedprefixes")
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

// TestNamespaceIdentity checks that a file whose stem cannot be a prefix, such
// as 2fa.go, is still a namespace: its test file shares it, a use from another
// namespace is reported, and the naming rule asks nothing of it.
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
// way Go does, that the prefix is matched ignoring case, and that the rename
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

// TestInterfaceMembers checks that an interface's method names are members.
// The silences carry as much as the reports: implicit satisfaction in impl.go
// is not a use, the type's directive and its ignore reach the methods, and an
// embedded interface or a type-constraint element declares no name to bound.
func TestInterfaceMembers(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "ifacemembers")
}

// TestPackageMain checks the two rules that only package main exercises: func
// main is the one name the toolchain requires, so the naming rules pass it by
// even at qualify: always, and a file-level scope directive reaches exported
// declarations, which is the only thing holding Config and ServerNew back.
func TestPackageMain(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "mainpkg")
}

// TestToolchainNames checks that a name the toolchain finds by name is never
// asked for a prefix. The silences carry the assertion: TestLoad, BenchmarkLoad,
// FuzzLoad and ExampleTestHelper would each be a function nothing runs once
// prefixed. TestHelper in a non-test file is still asked, because nothing
// collects it by name.
func TestToolchainNames(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "toolchainnames")
}

// TestSurplus checks the surplus rule's reporting shapes: one report per
// physical //declscope:package, listing every declaration that takes its
// scope from it, at every level the directive can be written. The quiet cases
// carry as much: an ignore silences it, and an exported name anywhere in the
// comment's reach keeps the whole comment.
func TestSurplus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplus")
}

// TestSurplusUsedOutside checks the first suppressor: a name spelled from
// another namespace keeps its directive, one member's use keeps a shared
// comment, a member that states its own scope stops carrying the type's, and
// a composite literal without keys counts as the use it is.
func TestSurplusUsedOutside(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusused")
}

// TestSurplusSatisfies checks the interface suppressor: a method reached only
// through a contract stays quiet, including through an anonymous struct, a
// function-local type, an instantiated generic type, a promoted
// pointer-receiver method, an anonymous interface in a type assertion, and a
// constraint carrying a type term. A method in no contract is still reported.
func TestSurplusSatisfies(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplussatisfies")
}

// TestSurplusCarrier checks the carrier suppressor: an exported type — or an
// exported alias, or an exported interface — carries an unexported method out
// of the package, where an importer can complete a satisfaction the analysis
// never sees. Only a method nothing exported carries is reported.
func TestSurplusCarrier(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surpluscarrier")
}

// TestSurplusLinkname checks the linkname suppressor: //go:linkname and
// //export name a declaration as text, from code the analysis does not read.
func TestSurplusLinkname(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surpluslinkname")
}

// TestSurplusOpaqueSource checks the opaque-source suppressor: a package
// holding a generated file has reference sites the analysis excludes, so the
// rule switches off for the package rather than read the absence as evidence.
// The silence is the assertion.
func TestSurplusOpaqueSource(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusopaque")
}

// TestSurplusSeesAllFiles checks that only a pass reading every file may
// report: the ordinary variant of a package with in-package tests switches the
// rule off, and the test variant sees the use. The silence in both variants is
// the assertion.
func TestSurplusSeesAllFiles(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplustests")
}

// TestSurplusConversion checks that a struct conversion counts as reach: it
// pairs every field by name and spells none of them, so a field whose
// directive holds the conversion together must stay unreported.
func TestSurplusConversion(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusconvert")
}

// TestSurplusDefaultOff pins the built-in default: with no config, the rule
// asks nothing.
func TestSurplusDefaultOff(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusdefault")
}

// TestSurplusBaseline checks that a recorded surplus finding is suppressed
// while a new one is still reported, through the same lookup as every rule.
func TestSurplusBaseline(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusbaselined")
}

// TestSurplusStrict checks strict's reporting shapes, at every level a
// directive encloses a declaration: a field under its type's directive, a
// declaration under its file's, a spec under its block's. Each is reported one
// per name when nothing outside reaches it, while its directive is otherwise in
// use. An entry holding a name that is read outside is not; neither is an
// exported declaration, one stating its own scope, an embedded field or a
// directive loose already reports. A type is reported with the members it
// would narrow, and not when one of them is reached. An ignore for surplus
// silences it, with the unused-ignore accounting.
func TestSurplusStrict(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusstrict")
}

// TestSurplusStrictReach checks every way a field is reached without its name
// being spelled from outside: a composite literal without keys, a selection
// promoted through an embedding, a selection on an instantiated generic type,
// a struct conversion, and a method written in another namespace. Each keeps
// the type's scope, and a field reached by none of them is still reported.
func TestSurplusStrictReach(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusstrictreach")
}

// TestSurplusStrictDefaults checks that under defaults.unexported: package
// strict adds nothing: the declaration would be package-scoped with no
// directive, so the directive widened nothing.
func TestSurplusStrictDefaults(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusstrictdefaults")
}

// TestSurplusLoose checks the default: a directive in use says nothing about
// the declarations under it that nothing outside reaches.
func TestSurplusLoose(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusloose")
}

// TestSurplusStrictSeesAllFiles checks that only a pass reading every file may
// report: the ordinary variant of a package with in-package tests switches the
// rule off, and the test variant sees the use a test file makes. What the
// test variant alone reports is pinned from a _test.go file.
func TestSurplusStrictSeesAllFiles(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusstricttests")
}

// TestSurplusStrictOpaqueSource checks that a package holding a generated
// file, whose reference sites the analysis excludes, gets no report.
func TestSurplusStrictOpaqueSource(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusstrictopaque")
}

// TestSurplusStrictBaseline checks that a recorded strict finding is
// suppressed while a new one is still reported, keyed like any surplus
// finding by the declaration's name.
func TestSurplusStrictBaseline(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "surplusstrictbaselined")
}

// TestDirectiveLoose checks the default rules.directive: a scope directive
// naming the scope defaults.unexported gives today is kept, since another
// configuration could make it bind, and one restating an enclosing directive
// is reported as before.
func TestDirectiveLoose(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "directiveloose")
}

// TestDirectiveStrict checks rules.directive: strict under the private
// default. A directive naming the scope its declarations would have without
// it is reported, at the field, type, block, spec and file levels, and one
// that widens, narrows, or is overridden to another effect is not. A block's
// directive judged only through the specs that override it is reported when
// they agree with it.
func TestDirectiveStrict(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "directivestrict")
}

// TestDirectiveStrictPackageDefault checks strict under defaults.unexported:
// package, where the same private field binds and a package directive on an
// unexported declaration is the redundant one.
func TestDirectiveStrictPackageDefault(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), declscope.Analyzer, "directivestrictpkg")
}
