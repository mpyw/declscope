package fixqualify

func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; rename it to userHelper, or to another name that carries "user"`

// The rename target is taken, so no fix is offered for this one.
func taken() int { return 2 } // want `func taken does not carry namespace "user" anywhere in its name; rename it to userTaken, or to another name that carries "user"`

func userTaken() int { return 3 }
