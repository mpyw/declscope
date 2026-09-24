//declscope:package // want `unused file-level //declscope:package: every declaration it reaches takes a nearer directive's scope or already has package scope`

package unusedstrictpkg

// One declaration takes the file's package, which the default gives anyway,
// and one states private. The report names both kinds.
func cWide() int { return 1 }

//declscope:private
func cNarrow() int { return 2 }

var _ = cWide() + cNarrow()
