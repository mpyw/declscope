package duplicatedirective

// A scope directive written twice is reported at the second. The first still
// decides the scope.
//
//declscope:shared
//declscope:shared // want `duplicate declscope:shared directive`
func userShared() int { return 2 }

//declscope:private
//declscope:private // want `duplicate declscope:private directive`
func UserPrivate() int { return 3 } // want `func UserPrivate is declared private by //declscope:private, but is used from namespace "order"`

// An ignore naming the same rules as one before it, in any order, is reported
// at the second and dropped. The first silences the use from order.go.
//
//declscope:ignore boundary,qualify
//declscope:ignore qualify,boundary // want `duplicate declscope:ignore directive`
func userIgnored() int { return 4 }

// The doc comment and a trailing comment are one binding, so a repeat across
// the two is a duplicate too.
//
//declscope:ignore boundary
func userTrailing() int { return 5 } //declscope:ignore boundary // want `duplicate declscope:ignore directive`

// Dropped, the duplicate is no second site for the unused report: the one
// mistake gets one report. The first ignore silences nothing and is reported
// on its own.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on userNeverLeaks`
//declscope:ignore boundary // want `duplicate declscope:ignore directive`
func userNeverLeaks() int { return 6 }

// An ignore naming a rule the other leaves out is no duplicate.
//
//declscope:ignore boundary
//declscope:ignore boundary,surplus
func userWider() int { return 7 }

var _ = userNeverLeaks
