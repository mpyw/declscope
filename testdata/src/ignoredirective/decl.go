package ignoredirective

// A malformed directive is answered by an ignore on the same declaration,
// where the two are still in hand together. Nothing is reported for either:
// the ignore silenced the report about the typo, so it is not unused.
//
//declscope:bogus
//declscope:ignore directive
func declTypo() int { return 3 }

// An ignore for the directive rule answers no unused report. That report is
// the unused rule's, so the ignore silenced nothing and is reported beside
// the directive it was written for.
//
//declscope:shared // want `unused //declscope:shared on DeclLeftover: nothing it reaches takes a scope`
//declscope:ignore directive // want `unused //declscope:ignore directive on DeclLeftover`
func DeclLeftover() int { return 4 }

// An ignore for the unused rule answers no directive report.
//
//declscope:bogus // want `unknown directive declscope:bogus`
//declscope:ignore unused // want `unused //declscope:ignore unused on declWrongRule`
func declWrongRule() int { return 5 }

// The doc comment and the trailing comment of an ungrouped type or var are one
// binding, as a func's are.
//
//declscope:bogus
type declTypoType int //declscope:ignore directive

//declscope:bogus
var declTypoVar declTypoType = 6 //declscope:ignore directive

// So are a block's doc comment and the comment after its "(".
//
//declscope:bogus
var ( //declscope:ignore directive
	declTypoBlock = 7
)

var _ = int(declTypoVar) + declTypoBlock + declTypo() + DeclLeftover() + declWrongRule()
