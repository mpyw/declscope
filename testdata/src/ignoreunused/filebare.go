//declscope:ignore // want `unused file-level //declscope:ignore`

package ignoreunused

// A bare file-level ignore covers every rule, the unused rule included, and
// still never its own report. Nothing here needs it, so it is reported.
func fileBare() int { return 1 }

var _ = fileBare()
