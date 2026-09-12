// Package declscope provides a go/analysis based analyzer that gives Go a
// third visibility level between exported and unexported.
//
// Go has two visibility levels, and the unexported one spans the whole
// package. In a package of any size that makes every helper, every field and
// every method a package-wide name, with no way to say "this belongs to this
// file" short of splitting the package. declscope adds file-private and
// package-internal as pseudo levels, carried by naming convention and by
// explicit directives, and enforces them statically.
package declscope

import (
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/mpyw/declscope/internal"
	"github.com/mpyw/declscope/internal/config"
)

// Analyzer is the declscope analyzer.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "declscope",
		Doc:  "enforces file-private and package-internal pseudo scopes for package-level declarations, methods and struct fields",
		URL:  "https://github.com/mpyw/declscope",
		Run:  run,
	}
	a.Flags.String("config", "", "path to a declscope YAML config file (default: nearest .declscope.yaml)")
	return a
}

func run(pass *analysis.Pass) (any, error) {
	opts, err := options(pass)
	if err != nil {
		return nil, err
	}
	return internal.Run(pass, opts)
}

// options resolves configuration for the package being analyzed. Config is
// looked up from the package's own directory upwards, so a subtree can relax
// or tighten the rules without affecting the rest of the module.
func options(pass *analysis.Pass) (internal.Options, error) {
	opts := internal.DefaultOptions()

	explicit := pass.Analyzer.Flags.Lookup("config").Value.String()
	path := explicit
	if path == "" {
		path = config.Find(packageDir(pass))
	}
	if path != "" {
		file, err := config.Load(path)
		if err != nil {
			return opts, fmt.Errorf("declscope: %w", err)
		}
		if err := file.Apply(&opts); err != nil {
			return opts, fmt.Errorf("declscope: %s: %w", path, err)
		}
	}
	if err := opts.Compile(); err != nil {
		return opts, fmt.Errorf("declscope: %w", err)
	}
	return opts, nil
}

func packageDir(pass *analysis.Pass) string {
	for _, f := range pass.Files {
		if pos := pass.Fset.Position(f.Pos()); pos.Filename != "" {
			return filepath.Dir(pos.Filename)
		}
	}
	return ""
}
