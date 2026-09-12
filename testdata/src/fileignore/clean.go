//declscope:ignore demote // want `unused file-level //declscope:ignore demote`

package fileignore

// Labelled, and demote is off, so the directive silences nothing.
func cleanOK() int { return 4 }

var _ = cleanOK
