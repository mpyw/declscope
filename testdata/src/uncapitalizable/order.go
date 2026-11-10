package uncapitalizable

// A namespace with a capital form is still held to the rule.
func Total() int { return 1 } // want `func Total does not carry namespace "order" anywhere in its name; give it a name that carries "order", such as OrderTotal, or move it to the file it belongs to`
