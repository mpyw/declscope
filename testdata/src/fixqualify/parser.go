package fixqualify

// parse is only a prefix of parser, a different word, so the name is
// prefixed whole: merging it would change what the name says.
func parseExpr() int { return 7 } // want `func parseExpr does not carry namespace "parser" anywhere in its name; rename it to parserParseExpr, or to another name that carries "parser"`
