package nsinitialism

// The second namespace. An initialism in a later segment is spelled the way
// Go does, so the prefix is parseJSON and parseJsonTree carries it too.
func parseJSONTree() int { return 1 }
func parseJsonTree() int { return 2 }

func tree() int { return 3 } // want `func tree does not carry namespace "parseJSON" anywhere in its name; give it a name that carries "parseJSON", such as parseJSONTree, or move it to the file it belongs to`

func ParseExported() int { return parseJSONTree() + parseJsonTree() + tree() }
