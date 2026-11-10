//declscope:namespace util

package fileignorescope

// The same namespace as util.go, but a file-level ignore belongs to the file
// that carries it, so this one is still asked for its prefix.
func extra() int { return 2 } // want `func extra does not carry namespace "util" anywhere in its name; move it to the file it belongs to, or give it a name that carries "util" \(utilExtra is one candidate, not necessarily the best\)`

var _ = extra
