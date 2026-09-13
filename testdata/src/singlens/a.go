//declscope:namespace shared

package singlens

// The package has a single namespace, so there is no boundary for a prefix to
// mark and the prefix is not required. Every other rule is structurally inert
// here too, since every reference is already inside the one namespace.
func helper() int { return 1 }

var count int
