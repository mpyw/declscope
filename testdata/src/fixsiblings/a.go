package fixsiblings

// Prefixing is not injective across namespaces: bX in namespace "a" and x in
// namespace "aB" both prefix to aBX. The first declaration in the pass claims
// the name; the second is reported without a fix.
func bX() int { return 1 } // want `func bX does not carry namespace "a" anywhere in its name; give it a name that carries "a", such as aBX, or move it to the file it belongs to`

var _ = bX
