package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/config"
	"github.com/mpyw/declscope/internal/rule"
	"github.com/mpyw/declscope/internal/shrink"
)

const baselineUsage = `Usage: declscope baseline [flags] [packages]

Records every current violation so that adopting declscope does not require
fixing them all at once. New violations are still reported.

Each package's entries go to the baseline the analyzer will consult for that
package: the one named by its nearest config file, else the nearest existing
default-named baseline at or below the working directory, else
` + baselineDefaultName + ` in the working directory. Every file written is
regenerated wholesale; baselines above the working directory are left alone.

What declscope shrink reports is recorded too, into the same files, so that
shrink passes on what the baseline already holds. That loads the whole module
once more. -shrink=false skips it, for a module that does not run shrink.

`

// baselineDefaultName is the file created when nothing else applies. It is
// the first of config.BaselineNames, spelled out so the usage text can be a
// constant.
const baselineDefaultName = ".declscope-baseline.yaml"

// baselineRun regenerates the baseline files from the current state of the
// code.
//
// It does not go through singlechecker, because a baseline entry has to
// identify a violation structurally — package, rule, declaration — and a
// driver only hands back rendered diagnostics. The analyzer requires only the
// inspector and exports no facts, so driving it over go/packages directly is a
// few lines and avoids parsing the analyzer's own messages back out of strings.
//
//declscope:shared
func baselineRun(args []string) {
	fs := flag.NewFlagSet("declscope baseline", flag.ExitOnError)
	out := fs.String("o", "", "write every entry to this one file instead")
	configPath := fs.String("config", "", "path to a declscope YAML config file")
	withShrink := fs.Bool("shrink", true, "also record what declscope shrink reports (-shrink=false skips it)")
	fs.Usage = func() {
		_, _ = io.WriteString(fs.Output(), baselineUsage)
		fs.PrintDefaults()
	}
	// ExitOnError: Parse reports a bad flag and exits with status 2 itself.
	_ = fs.Parse(args)

	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	cwd, err := os.Getwd()
	if err != nil {
		baselineFail(err)
	}
	targets, err := baselineCollect(patterns, *configPath, *out, cwd, *withShrink)
	if err != nil {
		baselineFail(err)
	}
	for _, path := range slices.Sorted(maps.Keys(targets)) {
		n, err := baseline.Save(path, targets[path])
		if err != nil {
			baselineFail(err)
		}
		fmt.Fprintf(os.Stderr, "declscope: recorded %d violation(s) in %s\n", n, path)
	}
}

// baselineCollect returns the current violations, grouped by the baseline file
// each belongs to. A file appears as a key as soon as one analyzed package resolves
// to it, even with no entries, so that regeneration prunes it.
//
// Resolving the target per package, rather than once for the run, is what
// makes "its presence is all it takes" true: the analyzer looks a baseline up
// from each package's own directory, so a subtree with its own config, or
// with its own default-named file, is suppressed only by entries written where
// that lookup ends. An explicit -o overrides this and gathers everything into
// one file, which is then the caller's job to place.
//
// With shrink, the reports of declscope shrink over the same patterns are
// placed the same way, by the package that declares each.
func baselineCollect(patterns []string, configPath, out, cwd string, shrinkToo bool) (map[string][]baseline.Key, error) {
	pkgs, err := loadPackages("baseline", patterns, true)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("packages contain errors")
	}

	targets := map[string][]baseline.Key{}
	if out != "" {
		targets[out] = nil
	}
	// Each package is resolved and collected on its own, in parallel. Only
	// the fold below sees more than one, and it runs in package order, so
	// the first error and the order of every entry are the ones a sequential
	// run would give.
	pkgs = slices.DeleteFunc(slices.Clone(pkgs), func(pkg *packages.Package) bool { return !loadIsAnalyzable(pkg) })
	type collectedPackage struct {
		path string // empty when no baseline is found from the package
		keys []baseline.Key
		err  error
	}
	results := make([]collectedPackage, len(pkgs))
	loadInParallel(len(pkgs), func(i int) {
		dir := loadedPackageDir(pkgs[i])
		// Options are resolved per package, since a subtree may configure its
		// own rules. The existing baseline is deliberately not loaded:
		// regeneration records the current state from scratch, and a file
		// that fails to parse must not block being replaced.
		opts, _, named, err := config.ResolveForBaseline(dir, configPath)
		if err != nil {
			results[i].err = err
			return
		}
		path := baselineTarget(named, dir, out, cwd)
		if path == "" {
			return
		}
		results[i] = collectedPackage{path: path, keys: internal.Collect(loadedPass(pkgs[i]), opts)}
	})

	var unplaceable []string
	for i, pkg := range pkgs {
		r := results[i]
		if r.err != nil {
			return nil, r.err
		}
		if r.path == "" {
			unplaceable = append(unplaceable, fmt.Sprintf("  %s (%s)", pkg.PkgPath, loadedPackageDir(pkg)))
			continue
		}
		targets[r.path] = append(targets[r.path], r.keys...)
	}
	if shrinkToo {
		res, err := shrink.Run(cwd, patterns, nil)
		if err != nil {
			return nil, fmt.Errorf("declscope shrink: %w (pass -shrink=false to record the analyzer's findings alone)", err)
		}
		for _, f := range res.Findings {
			// An unused ignore is a mistake in new code, not a finding the
			// codebase already has.
			if f.Rule != rule.Overexported {
				continue
			}
			// The analyzer's pass above resolved this directory already: a
			// config error there has returned, and a package no lookup would
			// find is among the unplaceable ones.
			dir := filepath.Dir(f.Pos.Filename)
			_, _, named, _ := config.ResolveForBaseline(dir, configPath)
			if path := baselineTarget(named, dir, out, cwd); path != "" {
				targets[path] = append(targets[path], f.Key())
			}
		}
	}
	if len(unplaceable) > 0 {
		slices.Sort(unplaceable)
		unplaceable = slices.Compact(unplaceable)
		return nil, fmt.Errorf("%s in %s would not be found from these packages, because the lookup from them never reaches the working directory:\n%s\nrun from a directory their lookup passes through, name a baseline in a config file near them, or pass -o",
			baselineDefaultName, cwd, strings.Join(unplaceable, "\n"))
	}
	return targets, nil
}

// baselineTarget is the file a package in dir records its entries in: out
// when given, else the baseline its nearest config names, else the file
// config.DefaultBaseline finds. It is empty when none would be found from dir.
func baselineTarget(named, dir, out, cwd string) string {
	if path := cmp.Or(out, named); path != "" {
		return path
	}
	path, _ := config.DefaultBaseline(dir, cwd)
	return path
}

func baselineFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope baseline: %v\n", err)
	os.Exit(1)
}
