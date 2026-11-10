// Package pattern answers, for every declscope command that takes package
// patterns, the question go/packages leaves out: which of the patterns named
// no package. The go command warns about each of those, and go vet stops when
// every pattern names nothing. A command that passed such a run silently
// would let a typo in CI check nothing. Resolve answers the rest of what
// declscope shrink asks of its patterns: which packages they name.
package pattern

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Unmatched returns the patterns that named no package, in order, given pkgs,
// the packages cfg loaded for all of them together. When none of them named
// any package, it returns an error in the go command's words instead.
//
// go/packages drops the go command's "matched no packages" warning, so only a
// load of each pattern on its own can tell which one it was about. That load
// asks for names alone, and is made only when there is a question to answer:
// with one pattern, or with nothing loaded, the answer is already known. A
// pattern the go command rejects yields a package holding the error, so it
// counts as matched here, and the caller reports the error.
func Unmatched(cfg *packages.Config, patterns []string, pkgs []*packages.Package) ([]string, error) {
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%s matched no packages", strings.Join(patterns, " "))
	}
	if len(patterns) == 1 {
		return nil, nil
	}
	names := &packages.Config{Dir: cfg.Dir, Env: cfg.Env, BuildFlags: cfg.BuildFlags, Mode: packages.NeedName}
	var unmatched []string
	for _, p := range patterns {
		one, err := packages.Load(names, p)
		if err != nil {
			return nil, err
		}
		if len(one) == 0 {
			unmatched = append(unmatched, p)
		}
	}
	return unmatched, nil
}

// Named is what the patterns name.
type Named struct {
	// Paths is every package they name in a main module.
	Paths map[string]bool
	// Outside counts the packages they name outside every main module: the
	// standard library, a dependency, anything all names.
	Outside int
	// Unmatched lists the patterns that named no package, in order.
	Unmatched []string
}

// Resolve resolves the patterns, from dir, to the packages they name, the way
// the go command does.
//
// A pattern the go command rejects is an error with the go command's own
// message, since go vet stops there too: a typo in CI must not pass as a run
// that found nothing. A pattern that names nothing is a warning, and an error
// when every pattern names nothing, which is what go vet does. Unmatched
// answers which ones those are, as it does for every command.
func Resolve(dir string, patterns []string) (Named, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	cfg := &packages.Config{Dir: dir, Mode: packages.NeedName | packages.NeedModule}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return Named{}, err
	}
	named := Named{Paths: map[string]bool{}}
	outside := map[string]bool{}
	var problems []string
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			problems = append(problems, p.Errors[0].Msg)
			continue
		}
		if p.Module != nil && p.Module.Main {
			named.Paths[p.PkgPath] = true
		} else {
			outside[p.PkgPath] = true
		}
	}
	if len(problems) > 0 {
		slices.Sort(problems)
		return Named{}, fmt.Errorf("%s", strings.Join(slices.Compact(problems), "\n"))
	}
	if named.Unmatched, err = Unmatched(cfg, patterns, pkgs); err != nil {
		return Named{}, err
	}
	named.Outside = len(outside)
	return named, nil
}
