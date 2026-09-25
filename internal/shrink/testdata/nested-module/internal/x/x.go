// Package x has a nested module under its parent, which could import it from
// a module this run never loads, so nothing is said.
package x

func Unused() {}
