package unusedstrict

// A redundant directive on a declaration another namespace uses is reported,
// but deleting it would move the level the boundary report names.
//
//declscope:private // want `unused //declscope:private on dShared: it already has private scope`
func dShared() int { return 1 } // want `func dShared is declared private by //declscope:private, but is used from namespace "e"`

// An ignore for the unused rule answers the strict report as well.
//
//declscope:private
//declscope:ignore unused
func dQuiet() int { return 2 }

var _ = dQuiet()
