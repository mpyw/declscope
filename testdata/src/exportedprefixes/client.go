package exportedprefixes

// The prefix carries the exportedness of the name it joins: New becomes
// ClientNew, never clientNew. A rename that quietly unexported a declaration
// would delete the package's API to satisfy a linter.
func New() int { return 1 } // want `func New does not carry namespace "client" anywhere in its name; rename it to ClientNew`

// No rename is offered for an exported declaration — its uses outside the
// package are never analyzed, so the rewrite could not be completed — so the
// suggested name in the message is all the author gets. fixexported pins the
// absence of the fix; here only the wording is checked.
var ErrClosed = 2 // want `var ErrClosed does not carry namespace "client" anywhere in its name; rename it to ClientErrClosed`

func helper() int { return 3 } // want `func helper does not carry namespace "client" anywhere in its name; rename it to clientHelper`

var _ = helper
