package fixqualify

type astFunctionCallExpr struct{}

// The name opens with the words that end the namespace, so the rename gains
// only the missing leading word rather than doubling the rest.
func callExprCache() int { return 5 } // want `func callExprCache does not carry namespace "astFunctionCallExpr" anywhere in its name; move it to the file it belongs to, or give it a name that carries "astFunctionCallExpr" \(astFunctionCallExprCache is one candidate, not necessarily the best\)`

// The name merges to the namespace, which the type above already holds, so
// the plain prefix is offered.
func expr() astFunctionCallExpr { return astFunctionCallExpr{} } // want `func expr does not carry namespace "astFunctionCallExpr" anywhere in its name; move it to the file it belongs to, or give it a name that carries "astFunctionCallExpr" \(astFunctionCallExprExpr is one candidate, not necessarily the best\)`
