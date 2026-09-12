package ignorescope

// Silencing one rule leaves the rest in force: the missing label is ignored,
// the boundary crossing is not.
//
//declscope:ignore promote
func helper() int { return 1 } // want `func helper is file-private to namespace "user", but is used from namespace "order"`

// Silencing everything.
//
//declscope:ignore
func helper2() int { return 2 }

// Two directives, each judged on its own: the label rule is silenced and so
// the first is used, while demote is inert in a package that requires the
// label, so the second is reported.
//
//declscope:ignore promote
//declscope:ignore demote // want `unused //declscope:ignore demote on kept`
func kept() int { return 3 } // want `func kept is file-private to namespace "user", but is used from namespace "order"`

// An unparseable directive silences nothing.
//
//declscope:ignore bogus // want `unknown rule "bogus" in declscope:ignore`
func userTypo() int { return 4 } // want `func userTypo is file-private to namespace "user", but is used from namespace "order"`

//declscope:ignore escape,promote
func helper3() int { return 5 }
