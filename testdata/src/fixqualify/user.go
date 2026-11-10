package fixqualify

func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userHelper is one candidate, not necessarily the best\)`

// The rename target is taken, so no fix is offered for this one.
func taken() int { return 2 } // want `func taken does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userTaken is one candidate, not necessarily the best\)`

func userTaken() int { return 3 }
