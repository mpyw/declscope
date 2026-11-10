package fixqualify

type astFunctionCallExpr struct{}

// The name opens with the words that end the namespace, so the rename gains
// only the missing leading word rather than doubling the rest.
func callExprCache() int { return 5 } // want `func callExprCache does not carry namespace "astFunctionCallExpr" anywhere in its name; give it a name that carries "astFunctionCallExpr", such as astFunctionCallExprCache, or move it to the file it belongs to`

// The name merges to the namespace, which the type above already holds, so
// the plain prefix is offered.
func expr() astFunctionCallExpr { return astFunctionCallExpr{} } // want `func expr does not carry namespace "astFunctionCallExpr" anywhere in its name; give it a name that carries "astFunctionCallExpr", such as astFunctionCallExprExpr, or move it to the file it belongs to`
