//declscope:package // want `unused file-level //declscope:package: every declaration it reaches takes a nearer directive's scope$`

package unusedstrictpkg

// The file's package restates the default, and its only declaration states
// private. The report must not say that bNarrow has package scope.
//
//declscope:private
func bNarrow() int { return 1 }

var _ = bNarrow()
