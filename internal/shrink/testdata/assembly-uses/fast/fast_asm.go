//go:build amd64 || arm64

package fast

// Fast returns cpu.HasFast.
func Fast() int64
