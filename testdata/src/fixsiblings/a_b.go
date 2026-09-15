package fixsiblings

func x() int { return 2 } // want `func x does not carry namespace "aB" anywhere in its name; rename it to aBX, or to another name that carries "aB"`

var _ = x
