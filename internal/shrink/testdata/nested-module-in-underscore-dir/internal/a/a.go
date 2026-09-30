// Package a is imported by the nested module in _tools, whose path lies under
// this module's. That module is loaded, so its use of F counts.
package a

func F() {}
