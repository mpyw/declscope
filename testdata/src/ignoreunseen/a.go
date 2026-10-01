package ignoreunseen

// aHelper is crossed into only by tagged.go, which the build leaves out. A
// configuration that reads that file reports the crossing, and the ignore
// answers it there, so it is not unused here.
//
//declscope:ignore boundary // tagged.go calls it behind the never tag
func aHelper() int { return 1 }

// A bare ignore covers boundary too.
//
//declscope:ignore
func aBare() int { return 2 }

// No unseen file names aStale, so its ignore silences nothing anywhere.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on aStale`
func aStale() int { return 3 }

// qualify is judged on the declaration, not on its uses, so an unseen file
// naming aQualify cannot make its ignore needed.
//
//declscope:ignore qualify // want `unused //declscope:ignore qualify on aQualify`
func aQualify() int { return 4 }

func A() int { return aHelper() + aBare() + aStale() + aQualify() }
