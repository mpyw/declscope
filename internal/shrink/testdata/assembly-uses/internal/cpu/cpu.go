// Package cpu holds a feature flag that assembly in another package reads,
// the way crypto assembly reads internal/cpu.X86.
package cpu

// HasFast is read only by fast/fast_amd64.s and fast/fast_arm64.s, which
// go/types never sees. The linker resolves the name, so it is a use.
var HasFast int64 = 1

// Spare is named by nothing, and is fixed.
var Spare int64 = 2 // want: var Spare is exported, but nothing outside \S+ uses it$
