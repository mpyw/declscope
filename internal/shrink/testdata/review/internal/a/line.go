package a

// A //line directive moves reported positions elsewhere. The fix must still
// edit this file.
//
//line fake.tmpl:1
func Lined() int { return 1 } // want: func Lined is exported, but nothing.*uses it$
