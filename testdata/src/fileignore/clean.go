//declscope:ignore qualify // want `unused file-level //declscope:ignore qualify`

package fileignore

// The name carries its namespace, so the directive silences nothing.
func cleanOK() int { return 4 }

var _ = cleanOK
