// A file-level ignore covering unused, in a file holding an ignore that names
// overexported, may be answering that ignore's unused report in declscope
// shrink, so it is not called unused here.
//
//declscope:ignore unused

package ignoremodulewide

//declscope:ignore overexported
func FileAnswered() int { return 5 }
