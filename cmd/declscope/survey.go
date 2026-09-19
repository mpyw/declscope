package main

import (
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope"
	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/config"
	"github.com/mpyw/declscope/internal/measure"
)

const surveyUsage = `Usage: declscope survey [flags] [packages]

Reports what was checked and what it found, one row per package, so that the
package to open first is the one at the top.

It decides nothing and gates nothing: the exit status is zero whatever the
counts say. Use declscope itself for that, and a baseline to adopt it.

`

// surveyRun measures every matching package.
//
//declscope:package // a subcommand, dispatched from main.go
func surveyRun(args []string) {
	fs := flag.NewFlagSet("declscope survey", flag.ExitOnError)
	configPath := fs.String("config", "", "path to a declscope YAML config file")
	allowErrors := fs.Bool("allow-errors", false, "measure anyway when some packages do not type-check")
	fs.Usage = func() {
		_, _ = io.WriteString(fs.Output(), surveyUsage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	pkgs, err := loadPackages(patterns)
	if err != nil {
		surveyFail(err)
	}
	failed := loadErrors(pkgs)
	if len(failed) > 0 && !*allowErrors {
		// A package that does not type-check produces no findings, and that
		// is indistinguishable from a package with nothing wrong. The skill
		// spends a section telling an agent to run go build first; a command
		// that reports counts should not need the warning.
		surveyFail(fmt.Errorf("these packages do not type-check, so their counts would be zero for a reason that is not the code:\n  %s\npass -allow-errors to measure the rest anyway",
			strings.Join(failed, "\n  ")))
	}

	summary, err := surveyPackages(pkgs, *configPath, failed)
	if err != nil {
		surveyFail(err)
	}
	if err := summary.WriteSummaryText(os.Stdout); err != nil {
		surveyFail(err)
	}
}

// surveyPackages measures each package under its own options, and gathers the
// state of the checks as it goes.
//
// Options are resolved per package because a subtree may configure its own
// rules, which is also why the report groups by the config chain that applied
// rather than naming one config for the run.
func surveyPackages(pkgs []*packages.Package, configPath string, failed []string) (measure.Summary, error) {
	var measured []measure.Package
	configs := map[string]*measure.ConfigUse{}
	var configOrder []string
	baselines := map[string]measure.BaselineUse{}
	analyzed := 0

	for _, pkg := range loadWidestVariants(pkgs) {
		dir := loadedPackageDir(pkg)
		opts, _, err := config.Resolve(dir, configPath)
		if err != nil {
			return measure.Summary{}, err
		}
		analyzed++

		chain := config.FindChain(dir)
		if configPath != "" {
			// An explicit -config builds no chain: the caller named the rules.
			chain = []string{configPath}
		}
		key := strings.Join(chain, "\x00")
		use, ok := configs[key]
		if !ok {
			use = &measure.ConfigUse{
				Chain:    chain,
				Boundary: !opts.AllowBoundary,
				Surplus:  !opts.AllowSurplus,
				Qualify:  opts.Qualify.String(),
				Exported: opts.NameExported,
			}
			configs[key] = use
			configOrder = append(configOrder, key)
		}
		use.Packages++

		if opts.BaselinePath != "" {
			baselines[opts.BaselinePath] = measure.BaselineUse{
				Path:    opts.BaselinePath,
				Entries: opts.Baseline.Len(),
			}
		}

		measured = append(measured, internal.Survey(&analysis.Pass{
			Analyzer:  declscope.Analyzer,
			Fset:      pkg.Fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report:    func(analysis.Diagnostic) {},
		}, opts))
	}

	checks := measure.Checks{
		TypeCheck: measure.TypeCheck{Packages: analyzed, Failed: failed},
	}
	for _, key := range configOrder {
		checks.Configs = append(checks.Configs, *configs[key])
	}
	for _, path := range slices.Sorted(maps.Keys(baselines)) {
		checks.Baselines = append(checks.Baselines, baselines[path])
	}
	return measure.SummaryOf(measured, checks), nil
}

func surveyFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope survey: %v\n", err)
	os.Exit(1)
}
