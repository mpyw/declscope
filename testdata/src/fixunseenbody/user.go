package fixunseenbody

// broken.go is excluded by the build and cannot be parsed, so the pass cannot
// tell whether it names helper or userHelper. The rename is withheld.
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userHelper, or move it to the file it belongs to`

var _ = helper
