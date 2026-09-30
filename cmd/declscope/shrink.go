package main

import (
	"cmp"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mpyw/declscope/internal/baseline"
	"github.com/mpyw/declscope/internal/config"
	"github.com/mpyw/declscope/internal/shrink"
)

const shrinkUsage = `Usage: declscope shrink [flags] [packages]

Reports the exported declarations of internal packages that nothing outside
their package uses, and with -fix unexports them.

Every package of the module is loaded, whatever the patterns name: an importer
outside them still counts. In a workspace, every module of it is loaded. The
patterns only choose which packages to report on, and default to ./... They
mean what they mean to the go command: a pattern it rejects stops the run, and
one that matches no package is warned about. Build tags come from GOFLAGS, as
for the analyzer.

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
	configPath := fs.String("config", "", "path to a declscope YAML config file, for the baseline it names")
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
	baselines := &shrinkBaselines{explicit: *configPath, sets: map[string]*baseline.Set{}}
	res, err := shrink.Run(dir, fs.Args(), baselines.has)
	if err == nil {
		err = baselines.err
	}
	if err != nil {
		shrinkFail(err)
	}
	// On stderr: a package not judged is no finding, but saying nothing
	// would read as nothing overexported there.
	for _, p := range res.Unmatched {
		fmt.Fprintf(os.Stderr, "declscope shrink: warning: %q matched no packages\n", p)
	}
	for _, s := range res.Skipped {
		fmt.Fprintf(os.Stderr, "declscope shrink: not judged: %s: %s\n", s.Package, s.Reason)
	}
	if res.Outside > 0 {
		fmt.Fprintf(os.Stderr, "declscope shrink: not judged: %d package(s) outside the main module\n", res.Outside)
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

// shrinkBaselines answers shrink.Baselined from the baseline that applies to
// each package, found as the analyzer finds it: named by the nearest config
// file, or the nearest default-named file. The first error resolving one
// fails the run, as it fails the analyzer.
type shrinkBaselines struct {
	explicit string
	sets     map[string]*baseline.Set
	err      error
}

func (b *shrinkBaselines) has(dir string, k baseline.Key) bool {
	set, ok := b.sets[dir]
	if !ok {
		opts, _, err := config.Resolve(dir, b.explicit)
		if err != nil {
			b.err = cmp.Or(b.err, err)
		}
		set = opts.Baseline
		b.sets[dir] = set
	}
	return set.Has(k)
}

func shrinkFail(err error) {
	fmt.Fprintf(os.Stderr, "declscope shrink: %v\n", err)
	os.Exit(1)
}
