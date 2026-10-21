//go:build declscope_never

package ignoreunseentest

// This test file is left out of the build. It crosses into a.go.
var _ = aHelper
