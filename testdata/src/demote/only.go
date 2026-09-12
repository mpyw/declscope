package demote

// The package has a single namespace, so the label is not required and must
// therefore not be there.
func onlyHelper() int { return 1 } // want `func onlyHelper carries the label of namespace "only", which is not required here; rename it to helper`

// An initialism left behind is spelled the way Go spells one.
var onlyID = 2 // want `var onlyID carries the label of namespace "only", which is not required here; rename it to id`

// Unlabelled, as it should be.
func helper2() int { return 3 }

// Not a word boundary, so the prefix was never a label.
func onlys() int { return 4 }

// Stripping this would leave a keyword, so it is left alone.
func onlyType() int { return 5 }

// Exported identifiers are never labelled.
func OnlyExported() int { return onlyHelper() + onlyID + helper2() + onlys() + onlyType() }
