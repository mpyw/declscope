package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mpyw/declscope/internal/shrink"
)

const shrinkUsage = `Usage: declscope shrink [flags] [packages]

Reports the exported declarations of internal packages that nothing outside
their package uses, and with -fix unexports them.

Every package of the module is loaded, whatever the patterns name: an importer
outside them still counts. The patterns only choose which packages to report
on, and default to ./...

A fix is offered only where no use can exist outside the package. Where a use
is possible but cannot be proved, such as a type that escapes into an
interface, the report stays and says why no fix is offered.

`

// shrinkRun reports, and with -fix unexports, what no importer uses.
//
//declscope:package // a subcommand, dispatched from main.go
func shrinkRun(args []string) {
	fs := flag.NewFlagSet("declscope shrink", flag.ExitOnError)
	fix := fs.Bool("fix", false, "unexport every declaration a fix is offered for")
	fs.Usage = func() {
		_, _ = io.WriteString(fs.Output(), shrinkUsage)
		fs.PrintDefaults()
	}
	// ExitOnError: Parse reports a bad flag and exits with status 2 itself.
	_ = fs.Parse(args)

	dir, err := os.Getwd()
	if err != nil {
		shrinkFail(err)
	}
	res, err := shrink.Run(dir, fs.Args())
	if err != nil {
		shrinkFail(err)
	}
	// On stderr: a package not judged is no finding, but saying nothing
	// would read as nothing overexported there.
	for _, s := range res.Skipped {
		fmt.Fprintf(os.Stderr, "declscope shrink: not judged: %s: %s\n", s.Package, s.Reason)
	}
	if *fix {
		if err := shrink.Apply(res.Findings); err != nil {
			shrinkFail(err)
		}
	}
	printed := 0
	for _, f := range res.Findings {
		if *fix && f.Fix != nil {
			continue
		}
		fmt.Printf("%s: %s\n", f.Pos, f.Message())
		printed++
	}
	// The analyzer's drivers exit 3 when they report, so a CI step treats the
	// two alike.
	if printed > 0 {
		os.Exit(3)
	}
}

func shrinkFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope shrink: %v\n", err)
	os.Exit(1)
}
