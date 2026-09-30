// Package q may be imported by the module in testdata/tools, one level
// below a directory ./... skips. That module is loaded, and imports nothing
// of this one, so q is judged.
package q

func F() {} // want: func F is exported, but nothing outside example.com/dp/internal/q uses it$
