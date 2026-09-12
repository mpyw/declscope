package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope"
	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/config"
)

// baselineDefaultName is used when no config file names one.
const baselineDefaultName = ".declscope-baseline.yaml"

const baselineUsage = `Usage: declscope baseline [flags] [packages]

Records every current violation so that adopting declscope does not require
fixing them all at once. New violations are still reported.

`

// baselineMain regenerates the baseline file from the current state of the
// code.
//
// It does not go through singlechecker, because a baseline entry has to
// identify a violation structurally — package, rule, declaration — and a
// driver only hands back rendered diagnostics. The analyzer declares no
// Requires and exports no facts, so driving it over go/packages directly is a
// few lines and avoids parsing our own messages back out of strings.
//
//declscope:package
func baselineMain(args []string) {
	fs := flag.NewFlagSet("declscope baseline", flag.ExitOnError)
	out := fs.String("o", "", "output path (default: the baseline named by the config file, else "+baselineDefaultName+")")
	configPath := fs.String("config", "", "path to a declscope YAML config file")
	fs.Usage = func() {
		_, _ = io.WriteString(fs.Output(), baselineUsage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	path, err := baselineOutputPath(*out, *configPath)
	if err != nil {
		baselineFail(err)
	}

	keys, err := baselineCollect(patterns, *configPath)
	if err != nil {
		baselineFail(err)
	}
	if err := baseline.Save(path, keys); err != nil {
		baselineFail(err)
	}
	fmt.Fprintf(os.Stderr, "declscope: recorded %d violation(s) in %s\n", len(keys), path)
}

// baselineOutputPath prefers an explicit -o, then the baseline named by the config
// nearest the working directory, then a default in the working directory.
func baselineOutputPath(out, configPath string) (string, error) {
	if out != "" {
		return out, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	opts, _, err := config.Resolve(dir, configPath)
	if err != nil {
		return "", err
	}
	if opts.BaselinePath != "" {
		return opts.BaselinePath, nil
	}
	return filepath.Join(dir, baselineDefaultName), nil
}

func baselineCollect(patterns []string, configPath string) ([]baseline.Key, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		// Test variants see references that the ordinary variant does not, and
		// a baseline that omitted them would report those as new.
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}

	var keys []baseline.Key
	for _, pkg := range pkgs {
		if len(pkg.Syntax) == 0 || pkg.TypesInfo == nil || pkg.Types == nil {
			continue
		}
		// Options are resolved per package, since a subtree may configure its
		// own rules. The baseline itself is deliberately not consulted here:
		// regeneration records the current state from scratch.
		opts, _, err := config.Resolve(baselinePackageDir(pkg), configPath)
		if err != nil {
			return nil, err
		}
		opts.BaselinePath, opts.Baseline = "", nil

		pass := &analysis.Pass{
			Analyzer:  declscope.Analyzer,
			Fset:      pkg.Fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report:    func(analysis.Diagnostic) {},
		}
		keys = append(keys, internal.Collect(pass, opts)...)
	}
	return keys, nil
}

func baselinePackageDir(pkg *packages.Package) string {
	for _, f := range pkg.GoFiles {
		return filepath.Dir(f)
	}
	for _, f := range pkg.CompiledGoFiles {
		return filepath.Dir(f)
	}
	return ""
}

func baselineFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope baseline: %v\n", err)
	os.Exit(1)
}
