package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope"
)

// loadPackages type-checks the patterns and hands back what the subcommands
// analyze.
//
// None of them goes through singlechecker: a baseline entry has to identify a
// violation structurally and a survey has to count what was suppressed, while
// a driver hands back rendered diagnostics. The analyzer declares no Requires
// and exports no facts, so driving it over go/packages directly is a few lines
// and avoids parsing the analyzer's own messages back out of strings.
//
// Tests are loaded by default, matching the analyzer's own -test: a test
// variant sees references the ordinary variant does not, and both the baseline
// and the survey would otherwise report a declaration as unreached when a test
// reaches it. Turning them off is how a reader asks what the package looks like
// without its scaffolding, which in a large package is most of what the
// crossings are.
//
//declscope:package // every subcommand that reads packages starts here
func loadPackages(patterns []string, tests bool) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedForTest,
		Tests: tests,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	return pkgs, nil
}

// loadErrors names every package that did not type-check.
//
// A failed build reports no diagnostics, and that is indistinguishable from a
// clean run: the adoption skill spends a section warning an agent to run
// go build first. A command that reads counts has to say so itself rather than
// leave the reader to notice, and it has to name the packages, which
// packages.PrintErrors only prints and counts.
//
// It reads every loaded package, not only the analyzable ones: a pattern
// naming a directory that does not exist yields a package with no syntax at
// all, and dropping it would answer "0 packages ok, 0 failed" to a question
// nothing could be measured for. One entry per import path, since the two
// variants of a package report the same error twice.
//
//declscope:package // survey refuses on it, and says which packages failed
func loadErrors(pkgs []*packages.Package) []string {
	seen := map[string]bool{}
	var failed []string
	for _, pkg := range pkgs {
		if loadIsTestMain(pkg) || len(pkg.Errors) == 0 || seen[pkg.PkgPath] {
			continue
		}
		seen[pkg.PkgPath] = true
		failed = append(failed, fmt.Sprintf("%s: %s", pkg.PkgPath, pkg.Errors[0]))
	}
	slices.Sort(failed)
	return failed
}

// loadErrorsForInspect leaves out an external test package when the package it
// tests is also present. inspect reports that subject package, not the separate
// scope declared by package foo_test, so an error in the skipped package must
// not prevent the requested package from being measured.
//
//declscope:package // inspect applies the loader's package-selection rules
func loadErrorsForInspect(pkgs []*packages.Package) []string {
	subjects := map[string]bool{}
	for _, pkg := range loadWidestVariants(pkgs) {
		if !loadIsExternalTest(pkg) {
			subjects[pkg.PkgPath] = true
		}
	}

	filtered := slices.DeleteFunc(slices.Clone(pkgs), func(pkg *packages.Package) bool {
		return loadIsExternalTest(pkg) && subjects[pkg.ForTest]
	})
	return loadErrors(filtered)
}

// loadIsAnalyzable reports whether a loaded package is one to analyze at all.
//
// The synthesized main package of a test binary lives in the build cache: no
// config or baseline could ever be looked up from it, and it has nothing of
// its own to say.
//
//declscope:package // both subcommands skip the same packages
func loadIsAnalyzable(pkg *packages.Package) bool {
	if len(pkg.Syntax) == 0 || pkg.TypesInfo == nil || pkg.Types == nil {
		return false
	}
	return !loadIsTestMain(pkg)
}

// loadIsTestMain identifies the synthetic executable built to run a package's
// tests. Depending on the go command's response, its file is either the
// generated _testmain.go or a build-cache archive; a real package is loaded
// from .go files even when its import path happens to end in ".test".
func loadIsTestMain(pkg *packages.Package) bool {
	if pkg.Name != "main" || !strings.HasSuffix(pkg.PkgPath, ".test") {
		return false
	}
	paths := append(slices.Clone(pkg.GoFiles), pkg.CompiledGoFiles...)
	for _, path := range paths {
		if filepath.Base(path) == "_testmain.go" {
			return true
		}
	}
	for _, path := range paths {
		// A real package is loaded from Go source. With the go command's
		// driver, the synthetic executable instead points at its build-cache
		// archive (a path ending in -d), even when NeedSyntax is requested.
		if filepath.Ext(path) == ".go" {
			return false
		}
	}
	return len(paths) > 0
}

// loadIsExternalTest identifies package foo_test as loaded for package foo.
// ForTest is the build system's answer; trimming an import-path suffix would
// also discard an ordinary package that is genuinely named foo_test.
//
//declscope:package // inspect excludes this separate scope when its subject is present
func loadIsExternalTest(pkg *packages.Package) bool {
	return pkg.ForTest != "" && pkg.PkgPath != pkg.ForTest && !loadIsTestMain(pkg)
}

// loadWidestVariants keeps one package per import path: the variant that sees
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
//declscope:package // survey measures one row per package, not per variant
func loadWidestVariants(pkgs []*packages.Package) []*packages.Package {
	widest := map[string]*packages.Package{}
	var paths []string
	for _, pkg := range pkgs {
		if !loadIsAnalyzable(pkg) {
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

// loadInParallel calls f once for every index below n, on up to GOMAXPROCS
// goroutines at a time, and returns once every call has.
//
// Loading type-checks in parallel already, and on a large module the analysis
// that follows is a quarter of the run: 0.7s of 2.5s over the standard
// library. Each caller writes one package's result to that package's own slot
// and folds the slots in order afterwards, so no call shares state with
// another and the output does not depend on scheduling. The analysis is safe
// to run this way: singlechecker already runs it on many packages at once.
//
//declscope:package // survey and baseline analyze every package through it
func loadInParallel(n int, f func(i int)) {
	slots := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := range n {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			f(i)
		})
	}
	wg.Wait()
}

// loadedPass builds the pass a subcommand analyzes a package with.
//
// OtherFiles and IgnoredFiles are not decoration. The surplus rule stands
// itself down for a package holding assembly, cgo or a build-excluded file,
// because it concludes from an absence and cannot see what those files use.
// A pass built without them answers that question wrongly, and the command
// reports a finding the analyzer itself refuses to print.
//
//declscope:package // every subcommand analyzes through this one pass
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
	}
}

// loadedPackageDir is the directory a package's config and baseline are looked
// up from.
//
//declscope:package // both subcommands resolve options per package
func loadedPackageDir(pkg *packages.Package) string {
	for _, f := range pkg.GoFiles {
		return filepath.Dir(f)
	}
	for _, f := range pkg.CompiledGoFiles {
		return filepath.Dir(f)
	}
	return ""
}
