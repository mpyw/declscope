package fixqualify

func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userHelper, or move it to the file it belongs to`

// The rename target is taken, so no fix is offered for this one.
func taken() int { return 2 } // want `func taken does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userTaken, or move it to the file it belongs to`

func userTaken() int { return 3 }
