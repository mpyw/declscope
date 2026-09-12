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

// The label is there and is not required, but no rename can be derived, so
// the violation is reported without one rather than passed over.
func onlyType() int { return 5 } // want `func onlyType carries the label of namespace "only", which is not required here, but "type" is a keyword; rename it by hand`

// The name is the namespace and nothing else.
func only() int { return 6 } // want `func only carries the label of namespace "only", which is not required here, but nothing would remain; rename it by hand`

// Exported identifiers are never labelled.
func OnlyExported() int { return onlyHelper() + onlyID + helper2() + onlys() + onlyType() + only() }
