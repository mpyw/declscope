package surplusbaselined

// userOld is recorded in the baseline, so it stays quiet.
//
//declscope:shared
func userOld() int { return 1 }

// userNew is not, so it is reported.
//
//declscope:shared // want `//declscope:shared on userNew: no use from another namespace is visible to declscope`
func userNew() int { return 2 }
