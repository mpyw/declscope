// Package a holds a value that the nested module in tools passes to
// q.Encode, a generic of this module. The nested module's load has q only
// as an import, which SSA builds no body for. The escape still counts, and
// Secret is not reported.
package a

type T struct{ Secret int }

func Make() T { return T{Secret: 1} }
