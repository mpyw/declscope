// Command declscope is a linter that enforces file-private and
// package-internal pseudo scopes within a Go package.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/mpyw/declscope"
)

func main() {
	singlechecker.Main(declscope.Analyzer)
}
