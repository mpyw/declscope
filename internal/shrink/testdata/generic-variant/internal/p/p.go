// Package p holds values that escape into json.Marshal inside q.Encode, a
// generic of the module. cmd/gv calls it through the variant of q without
// its tests, which is not the variant read, so SSA has no body for the
// instance. The escape still counts, and neither field is reported.
package p

type T struct{ Secret int }

func Make() T { return T{Secret: 1} }

// U reaches q.Encode only through s.Wrap, whose own instance has a body.
type U struct{ Hidden int }

func MakeU() U { return U{Hidden: 1} }
