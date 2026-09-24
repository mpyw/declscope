//declscope:ignore
//declscope:ignore boundary

package ignoreunused

// At the file level a bare ignore answers another's report, since it stands
// every rule down for the file. The second ignore silences nothing, the first
// answers its report, and neither is reported.
func filePair() int { return 1 }

var _ = filePair()
