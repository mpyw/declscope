package ignoreunused

// An ignore beside this one that names the unused rule answers for it: the
// author has already said that the unused report on this declaration's
// directives is not to be made. Answering it is a use.
//
//declscope:ignore qualify
//declscope:ignore unused
func declQuiet() int { return 1 }

// Without that sibling the same ignore is reported, which is what makes the
// line above a decision rather than a blanket.
//
//declscope:ignore qualify // want `unused //declscope:ignore qualify on declLoud`
func declLoud() int { return 2 }

// A bare ignore covers the unused rule, but on a declaration it answers no
// sibling: it would let a group of ignores exempt one another. Both are
// reported.
//
//declscope:ignore qualify // want `unused //declscope:ignore qualify on declBare`
//declscope:ignore // want `unused //declscope:ignore on declBare`
func declBare() int { return 3 }

// An ignore never answers its own report, whatever it names. One that could
// would never be called unused.
//
//declscope:ignore unused // want `unused //declscope:ignore unused on declSelf`
func declSelf() int { return 4 }

//declscope:ignore qualify,unused // want `unused //declscope:ignore qualify,unused on declBoth`
func declBoth() int { return 5 }

// The unused report on a scope directive is answered by an ignore naming the
// rule on the same declaration, and a bare one there answers it too.
//
//declscope:package
//declscope:ignore unused
func DeclNamed() int { return 6 }

//declscope:package
//declscope:ignore
func DeclBare() int { return 7 }

var _ = declQuiet() + declLoud() + declBare() + declSelf() + declBoth() + DeclNamed() + DeclBare()
