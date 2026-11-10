package fixsiblings

func x() int { return 2 } // want `func x does not carry namespace "aB" anywhere in its name; move it to the file it belongs to, or give it a name that carries "aB" \(aBX is one candidate, not necessarily the best\)`

var _ = x
