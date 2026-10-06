package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/config"
	"github.com/mpyw/declscope/internal/measure"
)

const inspectUsage = `Usage: declscope inspect [flags] <package>

Reports the shape of one package: its namespaces, every crossing between them,
what became of each crossing, and where the naming rule stands.

Takes one package. A namespace is implicitly qualified by its package, so
counts from two packages cannot be added up, and averaging them away silently
would answer a question nobody asked.

`

// inspectRun reports one package.
//
//declscope:package // a subcommand, dispatched from main.go
func inspectRun(args []string) {
	fs := flag.NewFlagSet("declscope inspect", flag.ExitOnError)
	configPath := fs.String("config", "", "path to a declscope YAML config file")
	tests := fs.Bool("test", true, "measure *_test.go files as well")
	format := fs.String("format", string(measure.FormatMarkdown), "output format: markdown or json")
	fs.Usage = func() {
		_, _ = io.WriteString(fs.Output(), inspectUsage)
		fs.PrintDefaults()
	}
	// ExitOnError: Parse reports a bad flag and exits with status 2 itself.
	_ = fs.Parse(args)
	patterns := fs.Args()
	if len(patterns) == 0 {
		fs.Usage()
		os.Exit(2)
	}
	chosen, err := measure.ParseFormat(*format)
	if err != nil {
		inspectFail(err)
	}

	if err := inspectRefuseManyBeforeLoad(patterns); err != nil {
		inspectFail(err)
	}

	pkgs, err := loadPackages("inspect", patterns, *tests)
	if err != nil {
		inspectFail(err)
	}
	if failed := loadErrorsForInspect(pkgs); len(failed) > 0 {
		// A package that does not type-check yields no findings, which reads
		// exactly like a package with nothing wrong. Refusing is the only
		// answer that cannot be misread.
		inspectFail(fmt.Errorf("these packages do not type-check, so nothing can be measured:\n  %s",
			strings.Join(failed, "\n  ")))
	}

	pkg, err := inspectOnePackage(loadWidestVariants(pkgs))
	if err != nil {
		inspectFail(err)
	}

	dir := loadedPackageDir(pkg)
	opts, _, err := config.Resolve(dir, *configPath)
	if err != nil {
		inspectFail(err)
	}
	surveyed := internal.Survey(loadedPass(pkg), opts)
	// The chain, not the nearest file. Config files compose key by key, so
	// the file next to the package can leave qualify at never while the one
	// above it turned the rule on, and naming only the nearer one
	// misattributes every rule that ran.
	surveyed.Config = config.FindChain(dir)
	if *configPath != "" {
		surveyed.Config = []string{*configPath}
	}
	if err := surveyed.WriteFormat(os.Stdout, chosen); err != nil {
		inspectFail(err)
	}
}

// inspectOnePackage picks the package to report, and refuses where the answer
// would be ambiguous.
//
// Its input is already one package per import path, the widest variant of
// each, so the ambiguity left is between distinct packages. An external test
// package is one of those: foo_test declares into its own scope with its own
// namespaces, and reporting it in answer to a request for foo would hand back
// a different package under the asked-for name. It is skipped when its
// subject is present, and refused alongside anything else.
func inspectOnePackage(pkgs []*packages.Package) (*packages.Package, error) {
	byPath := map[string]*packages.Package{}
	for _, pkg := range pkgs {
		byPath[pkg.PkgPath] = pkg
	}

	var paths []string
	for path := range byPath {
		pkg := byPath[path]
		if loadIsExternalTest(pkg) {
			if _, ok := byPath[pkg.ForTest]; ok {
				continue
			}
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)

	// No pattern can leave paths empty: loadPackages refuses one that names
	// no package, and a package that does not type-check is refused before
	// this.
	switch len(paths) {
	case 1:
		return byPath[paths[0]], nil
	default:
		return nil, inspectManyPackagesError(paths)
	}
}

// inspectRefuseManyBeforeLoad refuses patterns that name more than one package
// before the full load type-checks every one of them.
//
// Over ./... in a large module the full load takes seconds and ends in a
// refusal anyway, and where one of the packages does not type-check the
// refusal is about that package instead of the pattern. A load of names alone
// costs about a tenth of a second, so it is made only for patterns that can
// name more than one package: several patterns, a wildcard, or a meta
// pattern. A single plain path names at most one package and its test
// variants, so it goes straight to the full load.
//
// The load leaves tests out. An in-package test variant shares its package's
// path, and an external test package is skipped beside its subject, so
// neither changes the count. A package the go command could not list, such as
// a directory that does not exist, is not counted: the full load reports its
// error, which says more than a count would. inspectOnePackage still decides
// what the full load hands back.
func inspectRefuseManyBeforeLoad(patterns []string) error {
	if len(patterns) == 1 && !inspectPatternMayMatchMany(patterns[0]) {
		return nil
	}
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName}, patterns...)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var paths []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 || seen[pkg.PkgPath] {
			continue
		}
		seen[pkg.PkgPath] = true
		paths = append(paths, pkg.PkgPath)
	}
	if len(paths) <= 1 {
		return nil
	}
	slices.Sort(paths)
	return inspectManyPackagesError(paths)
}

// inspectPatternMayMatchMany reports whether one pattern can name more than one
// package: a wildcard, or one of the go command's meta patterns.
func inspectPatternMayMatchMany(p string) bool {
	switch p {
	case "all", "std", "cmd", "tool", "work":
		return true
	}
	return strings.Contains(p, "...")
}

func inspectManyPackagesError(paths []string) error {
	return fmt.Errorf("this pattern matches %d packages, and a namespace of one means nothing in another:\n  %s\nname one of them",
		len(paths), strings.Join(paths, "\n  "))
}

func inspectFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope inspect: %v\n", err)
	os.Exit(1)
}
