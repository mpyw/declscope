package surplusdefault

// The rule is on with no configuration.
//
//declscope:package // want `//declscope:package on userQuiet: no use from another namespace is visible to declscope`
func userQuiet() int { return 1 }
