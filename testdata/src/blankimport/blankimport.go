package blankimport

// driver.go declares nothing but an import, which registers whatever the
// imported package registers. That is a second namespace, so the rule applies.
func seed() int { return 1 } // want `func seed does not carry namespace "blankimport" anywhere in its name; move it to the file it belongs to, or give it a name that carries "blankimport" \(blankimportSeed is one candidate, not necessarily the best\)`

var _ = seed
