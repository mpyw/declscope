package uncapitalizable

// A namespace with a capital form is still held to the rule.
func Total() int { return 1 } // want `func Total does not carry namespace "order" anywhere in its name; move it to the file it belongs to, or give it a name that carries "order" \(OrderTotal is one candidate, not necessarily the best\)`
