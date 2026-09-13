package removedpublic

// A directive removed with the public scope is answered by name and offers to
// delete itself, so that `declscope -fix ./...` performs the upgrade. Every
// other directive problem gets no fix: what to write instead of a malformed one
// is the author's decision, and guessing there would rewrite intent rather than
// migrate it.
//
//declscope:public // want `//declscope:public was removed: an exported declaration carries no boundary, so the directive stated nothing — delete the line`
func Old() int { return 1 }
