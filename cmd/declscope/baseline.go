package main

import (
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/config"
)

const baselineUsage = `Usage: declscope baseline [flags] [packages]

Records every current violation so that adopting declscope does not require
fixing them all at once. New violations are still reported.

Each package's entries go to the baseline the analyzer will consult for that
package: the one named by its nearest config file, else the nearest existing
default-named baseline at or below the working directory, else
` + baselineDefaultName + ` in the working directory. Every file written is
regenerated wholesale; baselines above the working directory are left alone.

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
// driver only hands back rendered diagnostics. The analyzer declares no
// Requires and exports no facts, so driving it over go/packages directly is a
// few lines and avoids parsing the analyzer's own messages back out of strings.
//
//declscope:package
func baselineRun(args []string) {
	fs := flag.NewFlagSet("declscope baseline", flag.ExitOnError)
	out := fs.String("o", "", "write every entry to this one file instead")
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

	cwd, err := os.Getwd()
	if err != nil {
		baselineFail(err)
	}
	targets, err := baselineCollect(patterns, *configPath, *out, cwd)
	if err != nil {
		baselineFail(err)
	}
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "declscope: no packages matched, nothing recorded")
		return
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
func baselineCollect(patterns []string, configPath, out, cwd string) (map[string][]baseline.Key, error) {
	pkgs, err := loadPackages(patterns, true)
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
	var unplaceable []string
	for _, pkg := range pkgs {
		if !loadIsAnalyzable(pkg) {
			continue
		}
		dir := loadedPackageDir(pkg)
		// Options are resolved per package, since a subtree may configure its
		// own rules. The existing baseline is deliberately not loaded:
		// regeneration records the current state from scratch, and a file
		// that fails to parse must not block being replaced.
		opts, _, named, err := config.ResolveForBaseline(dir, configPath)
		if err != nil {
			return nil, err
		}
		path := out
		if path == "" {
			path = named
		}
		if path == "" {
			p, ok := config.DefaultBaseline(dir, cwd)
			if !ok {
				unplaceable = append(unplaceable, fmt.Sprintf("  %s (%s)", pkg.PkgPath, dir))
				continue
			}
			path = p
		}

		targets[path] = append(targets[path], internal.Collect(loadedPass(pkg), opts)...)
	}
	if len(unplaceable) > 0 {
		slices.Sort(unplaceable)
		unplaceable = slices.Compact(unplaceable)
		return nil, fmt.Errorf("%s in %s would not be found from these packages, because the lookup from them never reaches the working directory:\n%s\nrun from a directory their lookup passes through, name a baseline in a config file near them, or pass -o",
			baselineDefaultName, cwd, strings.Join(unplaceable, "\n"))
	}
	return targets, nil
}

func baselineFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope baseline: %v\n", err)
	os.Exit(1)
}
