package exportedprefixes

// The prefix carries the exportedness of the name it joins: New becomes
// ClientNew, never clientNew. A rename that quietly unexported a declaration
// would delete the package's API to satisfy a linter.
func New() int { return 1 } // want `func New does not carry namespace "client" anywhere in its name; move it to the file it belongs to, or give it a name that carries "client" \(ClientNew is one candidate, not necessarily the best\)`

// No rename is offered for an exported declaration — its uses outside the
// package are never analyzed, so the rewrite could not be completed — so the
// suggested name in the message is all the author gets. fixexported pins the
// absence of the fix; here only the wording is checked.
var ErrClosed = 2 // want `var ErrClosed does not carry namespace "client" anywhere in its name; move it to the file it belongs to, or give it a name that carries "client" \(ClientErrClosed is one candidate, not necessarily the best\)`

func helper() int { return 3 } // want `func helper does not carry namespace "client" anywhere in its name; move it to the file it belongs to, or give it a name that carries "client" \(clientHelper is one candidate, not necessarily the best\)`

var _ = helper
