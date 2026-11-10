package qualifyfixoptin

func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userHelper, or move it to the file it belongs to`
