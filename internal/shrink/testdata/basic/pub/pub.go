// Package pub is importable by other modules. Its exported API is where the
// exposure walk starts.
package pub

import "example.com/basic/internal/a"

// Unused is exported and unused, but outside internal/, so another module may
// import it and nothing is said.
func Unused() {}

// Get hands out a value of a.Exposed.
func Get() a.Exposed { return a.Exposed{} }
