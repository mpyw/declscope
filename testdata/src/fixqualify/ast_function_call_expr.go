package fixqualify

// The name opens with the words that end the namespace, so the rename gains
// only the missing leading word rather than doubling the rest.
func callExprCache() int { return 5 } // want `func callExprCache does not carry namespace "astFunctionCallExpr" anywhere in its name; rename it to astFunctionCallExprCache, or to another name that carries "astFunctionCallExpr"`
