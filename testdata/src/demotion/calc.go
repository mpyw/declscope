package demotion

// calcHelper claims package-wide reach through its prefix but never leaves its
// own namespace.
func calcHelper() int { return 1 } // want `func calcHelper is namespace-prefixed but is only used inside namespace "calc"; drop the prefix or state the scope`

// calcShared earns its prefix.
func calcShared() int { return 2 }

// calcRun earns its prefix.
func calcRun() int { return calcHelper() }

// calcUnused has no uses at all, which is a job for an unused-code linter.
func calcUnused() int { return 3 }

//declscope:package
func calcDeliberate() int { return 4 }

var _ = calcDeliberate
