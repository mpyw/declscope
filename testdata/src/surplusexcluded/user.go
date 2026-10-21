package surplusexcluded

// The only use is in order_tagged.go, which the build excludes. That file is
// of this package, so the rule switches off rather than read the absence as
// evidence. The silence is the assertion.
//
//declscope:shared
func userHelper() int { return 1 }
