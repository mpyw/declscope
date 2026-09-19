package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope"
	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/config"
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
	fs.Usage = func() {
		_, _ = io.WriteString(fs.Output(), inspectUsage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		fs.Usage()
		os.Exit(2)
	}

	pkgs, err := loadPackages(patterns)
	if err != nil {
		inspectFail(err)
	}
	if failed := loadErrors(pkgs); len(failed) > 0 {
		// A package that does not type-check yields no findings, which reads
		// exactly like a package with nothing wrong. Refusing is the only
		// answer that cannot be misread.
		inspectFail(fmt.Errorf("these packages do not type-check, so nothing can be measured:\n  %s",
			strings.Join(failed, "\n  ")))
	}

	pkg, err := inspectOnePackage(pkgs)
	if err != nil {
		inspectFail(err)
	}

	opts, configFile, err := config.Resolve(loadedPackageDir(pkg), *configPath)
	if err != nil {
		inspectFail(err)
	}
	surveyed := internal.Survey(&analysis.Pass{
		Analyzer:  declscope.Analyzer,
		Fset:      pkg.Fset,
		Files:     pkg.Syntax,
		Pkg:       pkg.Types,
		TypesInfo: pkg.TypesInfo,
		Report:    func(analysis.Diagnostic) {},
	}, opts)
	if configFile != "" {
		surveyed.Config = []string{configFile}
	}
	if err := surveyed.WriteText(os.Stdout); err != nil {
		inspectFail(err)
	}
}

// inspectOnePackage picks the package to report, and refuses where the answer
// would be ambiguous.
//
// Loading with tests turns one pattern into up to three packages: the package,
// its external test package, and the synthesized test binary. That is not the
// ambiguity worth refusing over — they are variants of one thing, and the one
// that sees every file is the one to report. Two distinct packages are.
func inspectOnePackage(pkgs []*packages.Package) (*packages.Package, error) {
	byPath := map[string][]*packages.Package{}
	var paths []string
	for _, pkg := range pkgs {
		if !loadIsAnalyzable(pkg) {
			continue
		}
		path := strings.TrimSuffix(pkg.PkgPath, "_test")
		if _, seen := byPath[path]; !seen {
			paths = append(paths, path)
		}
		byPath[path] = append(byPath[path], pkg)
	}
	switch len(paths) {
	case 0:
		return nil, fmt.Errorf("no package matched")
	case 1:
	default:
		slices.Sort(paths)
		return nil, fmt.Errorf("this pattern matches %d packages, and a namespace of one means nothing in another:\n  %s\nname one of them",
			len(paths), strings.Join(paths, "\n  "))
	}

	// The variant that sees the most files sees the test files too, and with
	// them the references the ordinary variant cannot: a declaration reached
	// only from a test is reached all the same.
	variants := byPath[paths[0]]
	widest := variants[0]
	for _, pkg := range variants[1:] {
		if len(pkg.Syntax) > len(widest.Syntax) {
			widest = pkg
		}
	}
	return widest, nil
}

func inspectFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope inspect: %v\n", err)
	os.Exit(1)
}
