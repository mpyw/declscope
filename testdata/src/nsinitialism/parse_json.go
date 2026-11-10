package nsinitialism

// The second namespace. An initialism in a later segment is spelled the way
// Go does, so the prefix is parseJSON and parseJsonTree carries it too.
func parseJSONTree() int { return 1 }
func parseJsonTree() int { return 2 }

func tree() int { return 3 } // want `func tree does not carry namespace "parseJSON" anywhere in its name; move it to the file it belongs to, or give it a name that carries "parseJSON" \(parseJSONTree is one candidate, not necessarily the best\)`

func ParseExported() int { return parseJSONTree() + parseJsonTree() + tree() }
