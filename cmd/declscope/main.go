// Command declscope is a linter that enforces private and
// package-internal pseudo scopes within a Go package.
//
// Usage:
//
//	declscope [flags] [packages]           analyze
//	declscope baseline [flags] [packages]  record current violations
//	declscope skill install                install the adoption skill
package main

import (
	"os"

	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/mpyw/declscope"
)

func main() {
	// Both subcommands are matched before singlechecker sees the arguments,
	// since singlechecker treats every non-flag argument as a package pattern.
	if len(os.Args) > 1 && os.Args[1] == "baseline" {
		baselineRun(os.Args[2:])
		return
	}
	skills.Intercept()
	// Before the driver: it registers a -V of its own only when nothing else
	// has, and the one it registers answers every binary with "devel".
	registerVersionFlag()
	singlechecker.Main(declscope.Analyzer)
}
