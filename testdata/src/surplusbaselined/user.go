package surplusbaselined

// userOld is recorded in the baseline, so it stays quiet.
//
//declscope:package
func userOld() int { return 1 }

// userNew is not, so it is reported.
//
//declscope:package // want `//declscope:package on userNew: no use from another namespace is visible to declscope`
func userNew() int { return 2 }
