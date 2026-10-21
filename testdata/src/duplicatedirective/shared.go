// A file-level scope written twice is reported at the second. The first still
// shares the file, so the use from order.go is not reported.
//
//declscope:shared
//declscope:shared // want `duplicate declscope:shared directive`

package duplicatedirective

func sharedHelper() int { return 1 }
