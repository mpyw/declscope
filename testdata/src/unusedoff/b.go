//declscope:ignore

package unusedoff

// A file-level ignore that silences nothing is not reported either.
func bQuiet() int { return 1 }

var _ = bQuiet()
