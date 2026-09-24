package fixunseenclause

// broken.go is excluded by the build and cannot be parsed, so the pass cannot
// tell whether it names helper or userHelper. The rename is withheld.
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; rename it to userHelper, or to another name that carries "user"`

var _ = helper
