package fixsiblings

func x() int { return 2 } // want `func x does not carry namespace "aB" anywhere in its name; give it a name that carries "aB", such as aBX, or move it to the file it belongs to`

var _ = x
