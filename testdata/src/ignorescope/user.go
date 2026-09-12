package ignorescope

// Silencing one rule leaves the rest in force: the missing label is ignored,
// the boundary crossing is not.
//
//declscope:ignore qualify
func helper() int { return 1 } // want `func helper is file-private to namespace "user", but is used from namespace "order"`

// Silencing everything.
//
//declscope:ignore
func helper2() int { return 2 }

// Two directives, each judged on its own: the label rule is silenced and so
// the first is used, while unqualify is inert in a package that requires the
// label, so the second is reported.
//
//declscope:ignore qualify
//declscope:ignore unqualify // want `unused //declscope:ignore unqualify on kept`
func kept() int { return 3 } // want `func kept is file-private to namespace "user", but is used from namespace "order"`

// An unparseable directive silences nothing.
//
//declscope:ignore bogus // want `unknown rule "bogus" in declscope:ignore`
func userTypo() int { return 4 } // want `func userTypo is file-private to namespace "user", but is used from namespace "order"`

//declscope:ignore boundary,qualify
func helper3() int { return 5 }

// Ignores accumulate across a block and its specs rather than the spec's
// replacing the block's: the bare ignore on the block keeps silencing boundary
// for userBlockB, and only its own unqualify — inert here — is unused.
//
//declscope:ignore
var (
	userBlockA = 6
	//declscope:ignore unqualify // want `unused //declscope:ignore unqualify on userBlockB`
	userBlockB = 7
)
