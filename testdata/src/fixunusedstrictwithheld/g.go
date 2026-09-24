package fixunusedstrictwithheld

// gShared is named by g_other.go, which the build excludes and the pass reads
// for its names alone: the crossing may be there.
//
//declscope:private // want `unused //declscope:private on gShared: it already has private scope`
func gShared() int { return 1 }
