//declscope:ignore unqualify // want `unused file-level //declscope:ignore unqualify`

package fileignore

// Prefixed, and unqualify is off, so the directive silences nothing.
func cleanOK() int { return 4 }

var _ = cleanOK
