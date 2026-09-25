// The same file-level ignore in a file with no such ignore answers nothing
// declscope shrink reports, and the analyzer judges it as always.
//
//declscope:ignore unused // want `unused file-level //declscope:ignore unused`

package ignoremodulewide

func Plain() int { return 6 }
