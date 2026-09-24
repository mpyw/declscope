//declscope:ignore unused
//declscope:package

package ignoreunused

// The file-level ignore above answers every unused report in the file: the
// file's own scope directive, which nothing takes since everything here is
// exported, and the declaration's ignore below, which silences nothing.
// Answering them is its use, so nothing in this file is reported.

//declscope:ignore boundary
func FileAnswered() int { return 1 }

var _ = FileAnswered()
