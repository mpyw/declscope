// Package a is not judged: the nested module in tools imports it but does
// not type-check, so whether it uses F is unknown.
package a

func F() {}
