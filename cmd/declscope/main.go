// Command declscope is a linter that enforces file-private and
// package-internal pseudo scopes within a Go package.
//
// Usage:
//
//	declscope [flags] [packages]           analyze
//	declscope baseline [flags] [packages]  record current violations
package main

import (
	"os"

	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/mpyw/declscope"
)

func main() {
	// The subcommand is matched before singlechecker sees the arguments, since
	// singlechecker treats every non-flag argument as a package pattern.
	if len(os.Args) > 1 && os.Args[1] == "baseline" {
		baselineMain(os.Args[2:])
		return
	}
	singlechecker.Main(declscope.Analyzer)
}
