package qualifyfixoptin

func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userHelper is one candidate, not necessarily the best\)`
