package blankimport

// driver.go declares nothing but an import, which registers whatever the
// imported package registers. That is a second namespace, so the rule applies.
func seed() int { return 1 } // want `func seed does not carry namespace "blankimport" anywhere in its name; rename it to blankimportSeed, or to another name that carries "blankimport"`

var _ = seed
