//declscope:ignore unused // want `unused file-level //declscope:ignore unused`

package ignoreunused

// A file-level ignore naming the unused rule answers the reports of the file,
// but not its own. Nothing else here needs it, so it is reported.
func fileSelf() int { return 1 }

var _ = fileSelf()
