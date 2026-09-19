// Package declscope provides a go/analysis based analyzer that gives Go
// pseudo visibility levels between exported and unexported.
//
// Go has two visibility levels, and the unexported one spans the whole
// package. In a package of any size that makes every helper, every field and
// every method a package-wide name, with no way to say "this belongs to this
// file" short of splitting the package. declscope adds private and
// package-internal as pseudo levels, selected by explicit directives and
// marked by a naming convention, and enforces them statically.
package declscope

import (
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
		Doc:  "enforces private and package-internal pseudo scopes for package-level declarations, methods and struct fields",
		URL:  "https://github.com/mpyw/declscope",
		Run:  analyzerRun,
	}
	a.Flags.String("config", "", "path to a declscope YAML config file (default: nearest .declscope.yaml)")
	return a
}

func analyzerRun(pass *analysis.Pass) (any, error) {
	opts, err := analyzerOptions(pass)
	if err != nil {
		return nil, err
	}
	return internal.Run(pass, opts)
}

// analyzerOptions resolves configuration for the package being analyzed. Config is
// looked up from the package's own directory upwards, so a subtree can relax
// or tighten the rules without affecting the rest of the module.
//
// The error is returned as is: the driver prefixes it with the analyzer's
// name when printing, so a "declscope: " prefix added here would appear twice.
func analyzerOptions(pass *analysis.Pass) (internal.Options, error) {
	explicit := pass.Analyzer.Flags.Lookup("config").Value.String()
	opts, _, err := config.Resolve(analyzerPackageDir(pass), explicit)
	return opts, err
}

func analyzerPackageDir(pass *analysis.Pass) string {
	for _, f := range pass.Files {
		// Unadjusted, for the same reason collect.go is: the config file is
		// looked up beside the file on disk, not beside a //line target.
		if pos := pass.Fset.PositionFor(f.Pos(), false); pos.Filename != "" {
			return filepath.Dir(pos.Filename)
		}
	}
	return ""
}
