package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope"
	"github.com/mpyw/declscope/internal/pattern"
	"github.com/mpyw/declscope/internal/shrink/module"
)

// loadPackages type-checks the patterns and hands back what the subcommands
// analyze.
//
// None of them goes through singlechecker: a baseline entry has to identify a
// violation structurally and a survey has to count what was suppressed, while
// a driver hands back rendered diagnostics. The analyzer requires only the
// inspector and exports no facts, so driving it over go/packages directly is a
// few lines and avoids parsing the analyzer's own messages back out of strings.
//
// Dependencies come from export data, as they do under singlechecker and go
// vet: with no facts, a driver type-checks only the packages it analyzes.
// Nothing here reads a dependency's syntax, and asking for NeedDeps had
// go/packages parse and type-check the standard library from source for it.
//
// Tests are loaded by default, matching the analyzer's own -test: a test
// variant sees references the ordinary variant does not, and both the baseline
// and the survey would otherwise report a declaration as unreached when a test
// reaches it. Turning them off is how a reader asks what the package looks like
// without its scaffolding, which in a large package is most of what the
// crossings are.
//
// Patterns that name no package are handled as go vet handles them: a
// warning for each, and an error when none names any. A baseline written from
// a typo would otherwise replace the real one with an empty file. command
// names the subcommand in the warning.
//
//declscope:shared // every subcommand that reads packages starts here
func loadPackages(command string, patterns []string, tests bool) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedForTest,
		Tests: tests,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	unmatched, err := pattern.Unmatched(cfg, patterns, pkgs)
	if err != nil {
		return nil, err
	}
	for _, p := range unmatched {
		fmt.Fprintf(os.Stderr, "declscope %s: warning: %q matched no packages\n", command, p)
	}
	return pkgs, nil
}

// loadFailures names every package that did not type-check.
//
// A failed build reports no diagnostics, and that is indistinguishable from a
// clean run: the authoring skill warns an agent to run
// go vet first. A command that reads counts has to say so itself rather than
// leave the reader to notice, and it has to name the packages, which
// packages.PrintErrors only prints and counts.
//
// It reads every loaded package, not only the analyzable ones: a pattern
// naming a directory that does not exist yields a package with no syntax at
// all, and dropping it would answer "0 packages ok, 0 failed" to a question
// nothing could be measured for. One entry per import path, since the two
// variants of a package report the same error twice.
//
// A package that compiles but imports one that does not is named too, along
// with the package that failed. loadFailureCauses says why it counts.
//
// Each entry names the failing package's first error that is not the go
// command's. Dependencies come from export data, so go list compiles the
// package too, and its build output comes first: a "-" position, a "# path"
// header and the compiler's message on a second line, relative to the working
// directory. The type checker reports the same error after it, on one line at
// an absolute position. A package the go command alone rejects, such as a
// directory that does not exist, has only the go command's error, and that one
// is named, on one line by module.ErrorLine.
//
//declscope:shared // survey and inspect refuse on it, and say which packages failed
func loadFailures(pkgs []*packages.Package) []string {
	causes := loadFailureCauses(pkgs)
	// The go command reports positions relative to its working directory,
	// which is this process's: the config sets no Dir.
	cwd, _ := os.Getwd()
	seen := map[string]bool{}
	var failed []string
	for _, pkg := range pkgs {
		cause := causes[pkg]
		if module.IsTestMain(pkg) || cause == nil || seen[pkg.PkgPath] {
			continue
		}
		seen[pkg.PkgPath] = true
		entry := pkg.PkgPath + ": "
		if cause != pkg {
			entry += "imports " + cause.PkgPath + ": "
		}
		failed = append(failed, entry+module.ErrorLine(cause, cwd))
	}
	slices.Sort(failed)
	return failed
}

// loadFailureCauses maps each package that cannot be measured to the package
// whose errors are why: itself, or the first package it imports, directly or
// not, that has errors.
//
// A package whose dependency does not compile still type-checks against the
// declarations the dependency claims, so its own clean result is not one. The
// analyzer's driver skips such a package, and baseline refuses it through
// packages.PrintErrors, which walks the imports the same way. Imports are
// visited in import-path order, so the dependency named does not depend on map
// order.
//
//declscope:shared // survey drops the packages it maps, and loadFailures names them
func loadFailureCauses(pkgs []*packages.Package) map[*packages.Package]*packages.Package {
	causes := map[*packages.Package]*packages.Package{}
	visited := map[*packages.Package]bool{}
	var visit func(pkg *packages.Package) *packages.Package
	visit = func(pkg *packages.Package) *packages.Package {
		if visited[pkg] {
			return causes[pkg]
		}
		visited[pkg] = true
		if len(pkg.Errors) > 0 {
			causes[pkg] = pkg
			return pkg
		}
		for _, path := range slices.Sorted(maps.Keys(pkg.Imports)) {
			if cause := visit(pkg.Imports[path]); cause != nil {
				causes[pkg] = cause
				return cause
			}
		}
		return nil
	}
	for _, pkg := range pkgs {
		visit(pkg)
	}
	return causes
}

// loadedAnalyzable reports whether a loaded package is one to analyze at all.
//
// The synthesized main package of a test binary lives in the build cache: no
// config or baseline could ever be looked up from it, and it has nothing of
// its own to say.
//
//declscope:shared // both subcommands skip the same packages
func loadedAnalyzable(pkg *packages.Package) bool {
	if len(pkg.Syntax) == 0 || pkg.TypesInfo == nil || pkg.Types == nil {
		return false
	}
	return !module.IsTestMain(pkg)
}

// loadedTestOnlyPaths names every package that has only test files, loaded
// without its tests.
//
// The go command lists such a directory as a package with no files and no
// error, so nothing marks it: it is not analyzable, and it is not a failure.
// A command that measures only what is analyzable would leave it out of the
// report without a word, so the caller has to name it. A directory of other
// files that cannot build, such as one of assembly alone or one excluded by
// build constraints, comes with an error, and the error is reported instead.
//
//declscope:shared // inspect refuses on it, and survey names what it skipped
func loadedTestOnlyPaths(pkgs []*packages.Package) []string {
	analyzable := map[string]bool{}
	for _, pkg := range pkgs {
		if loadedAnalyzable(pkg) {
			analyzable[pkg.PkgPath] = true
		}
	}
	seen := map[string]bool{}
	var paths []string
	for _, pkg := range pkgs {
		if module.IsTestMain(pkg) || len(pkg.Errors) > 0 || len(pkg.GoFiles) > 0 || analyzable[pkg.PkgPath] || seen[pkg.PkgPath] {
			continue
		}
		seen[pkg.PkgPath] = true
		paths = append(paths, pkg.PkgPath)
	}
	slices.Sort(paths)
	return paths
}

// loadedExternalTest identifies package foo_test as loaded for package foo.
// ForTest is the build system's answer; trimming an import-path suffix would
// also discard an ordinary package that is genuinely named foo_test.
//
//declscope:shared // inspect excludes this separate scope when its subject is present
func loadedExternalTest(pkg *packages.Package) bool {
	return pkg.ForTest != "" && pkg.PkgPath != pkg.ForTest && !module.IsTestMain(pkg)
}

// loadedWidestVariants keeps one package per import path: the variant that sees
// the most files.
//
// Loading with tests yields up to three packages for one directory, and two of
// them carry the same import path — the package, and the package plus its
// in-package test files. Measuring both would count every finding twice, and
// the one to keep is the wider: a declaration reached only from a test is
// reached all the same, and the variant that cannot see the test file cannot
// see that.
//
// An external test package keeps its own row. Its import path ends in _test
// because it is a different package, with its own namespaces and its own
// scope, and folding it into its subject would mix two packages' counts.
//
//declscope:shared // survey and inspect measure one row per package, not per variant
func loadedWidestVariants(pkgs []*packages.Package) []*packages.Package {
	widest := map[string]*packages.Package{}
	var paths []string
	for _, pkg := range pkgs {
		if !loadedAnalyzable(pkg) {
			continue
		}
		seen, ok := widest[pkg.PkgPath]
		if !ok {
			paths = append(paths, pkg.PkgPath)
		}
		if !ok || len(pkg.Syntax) > len(seen.Syntax) {
			widest[pkg.PkgPath] = pkg
		}
	}
	slices.Sort(paths)
	out := make([]*packages.Package, 0, len(paths))
	for _, path := range paths {
		out = append(out, widest[path])
	}
	return out
}

// loadedPass builds the pass a subcommand analyzes a package with.
//
// OtherFiles and IgnoredFiles are not decoration. The surplus rule stands
// itself down for a package holding assembly, cgo or a build-excluded file,
// because it concludes from an absence and cannot see what those files use.
// A pass built without them answers that question wrongly, and the command
// reports a finding the analyzer itself refuses to print.
//
// ResultOf holds what a driver would have run first: the analyzer's one
// requirement, the inspector over the same files.
//
//declscope:shared // every subcommand analyzes through this one pass
func loadedPass(pkg *packages.Package) *analysis.Pass {
	return &analysis.Pass{
		Analyzer:     declscope.Analyzer,
		Fset:         pkg.Fset,
		Files:        pkg.Syntax,
		OtherFiles:   pkg.OtherFiles,
		IgnoredFiles: pkg.IgnoredFiles,
		Pkg:          pkg.Types,
		TypesInfo:    pkg.TypesInfo,
		Report:       func(analysis.Diagnostic) {},
		ResultOf: map[*analysis.Analyzer]any{
			inspect.Analyzer: inspector.New(pkg.Syntax),
		},
	}
}

// loadedPackageDir is the directory a package's config and baseline are looked
// up from.
//
//declscope:shared // baseline, inspect and survey resolve options per package
func loadedPackageDir(pkg *packages.Package) string {
	for _, f := range slices.Concat(pkg.GoFiles, pkg.CompiledGoFiles) {
		return filepath.Dir(f)
	}
	// A package with no file at all. go list reports one only with an error,
	// and every caller drops those first.
	return ""
}
