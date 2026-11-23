// Package x is used only by its external tests, and those tests are behind a
// build tag. The run that leaves them out reads them as external tests too,
// so every report reads the same in both runs, and one ignore passes both.
package x

// Helper is used only by x_tagged_test.go.
func Helper() int { return 1 } // want: func Helper is exported, but only the external tests of \S+ use it \(no fix: external tests use it\)$

// Kept is used only by x_tagged_test.go, and its ignore is needed in both
// runs.
//
//declscope:ignore overexported // only the tagged tests use it
func Kept() int { return 2 }

// T is handed out by New, which keeps its name: only external tests call it.
type T struct{} // want: type T is exported.*\(no fix: an exported declaration that keeps its name hands it out\)$

// New is used only by x_tagged_test.go.
func New() T { return T{} } // want: func New is exported, but only the external tests of \S+ use it \(no fix: external tests use it\)$
