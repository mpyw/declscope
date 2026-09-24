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
//declscope:package // want `unused //declscope:package on DeclLeftover: nothing it reaches takes a scope`
//declscope:ignore directive // want `unused //declscope:ignore directive on DeclLeftover`
func DeclLeftover() int { return 4 }

// An ignore for the unused rule answers no directive report.
//
//declscope:bogus // want `unknown directive declscope:bogus`
//declscope:ignore unused // want `unused //declscope:ignore unused on declWrongRule`
func declWrongRule() int { return 5 }

var _ = declTypo() + DeclLeftover() + declWrongRule()
