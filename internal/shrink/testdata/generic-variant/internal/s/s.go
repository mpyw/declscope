// Package s has no test, so its variant read is the one cmd/gv imports, and
// SSA builds Wrap's instance. The call to q.Encode inside it still goes to
// the variant of q without its tests, and is given a type parameter, which
// names no type of p.
package s

import "example.com/gv/internal/q"

func Wrap[V any](v V) string { return q.Encode(v) }
