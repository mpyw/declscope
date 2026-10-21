//go:build declscope_never

package fixtesttagged

// This test file belongs to the package, but no build configuration the
// analysis runs under includes it. It names load.

var _ = load
