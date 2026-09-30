// Package x has a nested module under its parent. That module is loaded, and
// imports nothing of this one, so x is judged like any other package.
package x

func Unused() {} // want: func Unused is exported, but nothing outside example.com/nested/internal/x uses it$
