package unqualify

// The package has a single namespace, so the prefix is not required and must
// therefore not be there.
func onlyHelper() int { return 1 } // want `func onlyHelper carries the prefix of namespace "only", which is not required here; rename it to helper`

// An initialism left behind is spelled the way Go spells one.
var onlyID = 2 // want `var onlyID carries the prefix of namespace "only", which is not required here; rename it to id`

// Unprefixed, as it should be.
func helper2() int { return 3 }

// Not a word boundary, so the prefix was never a prefix.
func onlys() int { return 4 }

// The prefix is there and is not required, but no rename can be derived, so
// the violation is reported without one rather than passed over.
func onlyType() int { return 5 } // want `func onlyType carries the prefix of namespace "only", which is not required here, but "type" is a keyword; rename it by hand`

// The name is the namespace and nothing else, so it carries no prefix to drop:
// the file is named after what it declares, not the other way about.
func only() int { return 6 }

// Exported identifiers are never prefixed.
func OnlyExported() int { return onlyHelper() + onlyID + helper2() + onlys() + onlyType() + only() }
