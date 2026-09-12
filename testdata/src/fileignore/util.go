//declscope:ignore qualify

package fileignore

// The file stands outside the label rule, so neither of these is asked to
// carry the "util" prefix.
func helper() int { return 1 }

// The boundary still holds: only qualify was silenced.
func helper2() int { return 2 } // want `func helper2 is file-private to namespace "util", but is used from namespace "order"`

var _ = helper
