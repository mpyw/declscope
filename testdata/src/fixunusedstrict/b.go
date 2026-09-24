package fixunusedstrict

// bShared is used from c.go, so deleting its directive would move the level
// the boundary report names. It is reported without a fix, and this file takes
// no edit.
//
//declscope:private // want `unused //declscope:private on bShared: it already has private scope`
func bShared() int { return 1 } // want `func bShared is declared private by //declscope:private, but is used from namespace "c"`
