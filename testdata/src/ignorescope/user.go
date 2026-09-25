package ignorescope

// Silencing one rule leaves the rest in force: the missing prefix is ignored,
// the boundary crossing is not.
//
//declscope:ignore qualify
func helper() int { return 1 } // want `func helper is private to namespace "user", but is used from namespace "order"`

// Silencing everything.
//
//declscope:ignore
func helper2() int { return 2 }

// Two directives, each judged on its own: the crossing is silenced and so the
// first is used, while qualify never fires on a name that carries its
// namespace, so the second is reported.
//
//declscope:ignore boundary
//declscope:ignore qualify // want `unused //declscope:ignore qualify on userKept`
func userKept() int { return 3 }

// An unparseable directive silences nothing.
//
//declscope:ignore bogus // want `unknown rule "bogus" in declscope:ignore \(want one of boundary, qualify, surplus, unused, directive, filter, overexported\)`
func userTypo() int { return 4 } // want `func userTypo is private to namespace "user", but is used from namespace "order"`

//declscope:ignore boundary,qualify
func helper3() int { return 5 }

// Ignores accumulate across a block and its specs rather than the spec's
// replacing the block's: the bare ignore on the block keeps silencing boundary
// for userBlockB, and only its own qualify — needless on a prefixed name — is
// unused.
//
//declscope:ignore
var (
	userBlockA = 6
	//declscope:ignore qualify // want `unused //declscope:ignore qualify on userBlockB`
	userBlockB = 7
)
