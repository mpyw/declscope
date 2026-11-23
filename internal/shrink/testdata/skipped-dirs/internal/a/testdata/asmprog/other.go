//go:build !amd64 && !arm64

package main

// read stands in for the assembly on other architectures.
func read() int64 { return 6 }
