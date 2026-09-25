package ignoremodulewide

// An ignore naming overexported silences nothing this analyzer reports, and
// it may be doing its job in declscope shrink, which this pass cannot see. It
// is not called unused here.
//
//declscope:ignore overexported
func Shrunk() int { return 1 }

// Naming another rule beside it does not make the analyzer the judge either:
// the part it cannot see may be the part in use.
//
//declscope:ignore overexported,qualify
func Mixed() int { return 2 }

// A bare ignore does not reach overexported, so the analyzer can judge it
// alone, and one that silenced nothing it reports is unused.
//
//declscope:ignore // want `unused //declscope:ignore on Bare`
func Bare() int { return 3 }

// An ignore naming unused beside one naming overexported may be answering
// the unused report declscope shrink makes, which this pass cannot see, so it
// is not called unused here either.
//
//declscope:ignore overexported
//declscope:ignore unused
func Answering() int { return 4 }
