//declscope:shared

package fixsurplusstrict

// memberHelper takes the file's package, and order.go calls it, so the file's
// directive is in use.
func memberHelper() int { return memberBox{}.count }

// memberBox takes it too, and no other namespace names it. Its field states
// //declscope:private, which binds only while the type is shared. Narrowing
// the type would leave that directive restating the type's scope, and the
// unused rule would report it. The directive is the author's, so the fix is
// withheld, and the report is not. The file takes no edit.
type memberBox struct { // want `type memberBox takes shared scope from the file's //declscope:shared, but no use from another namespace is visible to declscope`
	//declscope:private
	count int
}
