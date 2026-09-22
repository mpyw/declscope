package ignoredirective

// An ignore written beside this one, in the same comment group, answers for
// it: the author has already said that reports about the directives on this
// declaration are not to be made, and the unused-ignore report is one of them.
//
//declscope:ignore qualify
//declscope:ignore directive
func declQuiet() int { return 1 }

// Without that sibling the same ignore is reported, which is what makes the
// line above a decision rather than a blanket.
//
//declscope:ignore qualify // want `unused //declscope:ignore qualify on declLoud`
func declLoud() int { return 2 }

var _ = declQuiet() + declLoud()

// A malformed directive is answered by an ignore on the same declaration,
// where the two are still in hand together. Nothing is reported for either:
// the ignore silenced the report about the typo, so it is not unused.
//
//declscope:bogus
//declscope:ignore directive
func declTypo() int { return 3 }

var _ = declTypo
