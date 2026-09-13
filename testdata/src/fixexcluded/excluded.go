//go:build declscope_never

package fixexcluded

// This file belongs to the package but no build configuration the analysis
// runs under includes it, so it is absent from pass.Files and the fix never
// rewrites it. It names load, and it declares userSpare.

func userSpare() int { return 0 }

func excludedUse() int { return load() + userSpare() }
