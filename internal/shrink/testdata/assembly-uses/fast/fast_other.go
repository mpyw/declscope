//go:build !amd64 && !arm64

package fast

// Fast stands in for the assembly on other architectures.
func Fast() int64 { return 1 }
