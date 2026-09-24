//declscope:package // want `unused file-level //declscope:package$`

package unusedscope

// A file-level scope is the default for what the file declares, and every
// declaration here is exported: an exported declaration is package-internal
// under every configuration, so nothing below takes its scope from the line
// above. The file-level report names no declaration, since the directive was
// written about the file rather than about any one of them.
func Widget() int { return 1 }

type Crate struct{ Name string }
