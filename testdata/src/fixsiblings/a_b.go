package fixsiblings

func x() int { return 2 } // want `func x does not carry the prefix of namespace "aB"; rename it to aBX`

var _ = x
