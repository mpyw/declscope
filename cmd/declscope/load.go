package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
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
// Tests are always loaded. A test variant sees references the ordinary variant
// does not, and both the baseline and the survey would otherwise report a
// declaration as unreached when a test reaches it.
//
//declscope:package // every subcommand that reads packages starts here
func loadPackages(patterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Tests: true,
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
//declscope:package // survey refuses on it, and says which packages failed
func loadErrors(pkgs []*packages.Package) []string {
	var failed []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) == 0 {
			continue
		}
		failed = append(failed, fmt.Sprintf("%s: %s", pkg.PkgPath, pkg.Errors[0]))
	}
	return failed
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
	return !(pkg.Name == "main" && strings.HasSuffix(pkg.PkgPath, ".test"))
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
