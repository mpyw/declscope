// Package a is not judged: the nested module in tools reads example.com/ne
// from nested-module-elsewhere-copy, so its uses are of other source.
package a

func F() {}
