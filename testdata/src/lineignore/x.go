//declscope:ignore

package lineignore

// A //line directive renames the positions below it. The file-level ignore
// above still covers them, because it is about the file on disk.
//
//line x.tmpl:1
//declscope:bogus
func run() {}
