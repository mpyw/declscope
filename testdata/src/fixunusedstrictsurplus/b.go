package fixdirectivestrictsurplus

// An ignore answers surplus for bQuiet, and would answer nothing once the
// directive is gone.
//
//declscope:package // want `unused //declscope:package on bQuiet: it already has package scope`
//declscope:ignore surplus
func bQuiet() int { return 1 }

var _ = bQuiet()
