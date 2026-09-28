//go:build declscope_never

package fixtestexcluded

// This file belongs to the package, but no build configuration the analysis
// runs under includes it. It names load.

func excludedUse() int { return load() }
