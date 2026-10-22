package uncapitalizable

// A namespace with a capital form is still held to the rule.
func Total() int { return 1 } // want `func Total does not carry namespace "order" anywhere in its name; rename it to OrderTotal, or to another name that carries "order"`
