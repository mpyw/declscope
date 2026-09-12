package fixsiblings

// Labeling is not injective across namespaces: bX in namespace "a" and x in
// namespace "aB" both label to aBX. The first declaration in the pass claims
// the name; the second is reported without a fix.
func bX() int { return 1 } // want `func bX does not carry the prefix of namespace "a"; rename it to aBX`

var _ = bX
