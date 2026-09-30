// Package pattern answers, for every declscope command that takes package
// patterns, the question go/packages leaves out: which of the patterns named
// no package. The go command warns about each of those, and go vet stops when
// every pattern names nothing. A command that passed such a run silently
// would let a typo in CI check nothing.
package pattern

import (
	"fmt"
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
