package surplusdefault

// The rule is on with no configuration.
//
//declscope:shared // want `//declscope:shared on userQuiet: no use from another namespace is visible to declscope`
func userQuiet() int { return 1 }
