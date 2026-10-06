package fixunusedstrictsurplus

// An ignore answers surplus for bQuiet, and would answer nothing once the
// directive is gone.
//
//declscope:shared // want `unused //declscope:shared on bQuiet: it already has shared scope`
//declscope:ignore surplus
func bQuiet() int { return 1 }

var _ = bQuiet()
