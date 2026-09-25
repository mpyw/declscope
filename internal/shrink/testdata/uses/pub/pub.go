// Package pub is importable by other modules. Its exported API is where the
// exposure walk starts.
package pub

import "example.com/uses/internal/a"

// Unused is exported and unused, but outside internal/, so another module may
// import it and nothing is said.
func Unused() {}

func Get() a.Exposed { return a.Exposed{} }

type Wrapper struct{ a.Promoted }

func GetWrapper() Wrapper { return Wrapper{} }

func GetOuter() a.Outer { return a.Outer{} }

type Iface interface{ Get() a.Out }

var K a.Kept
