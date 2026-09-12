//declscope:namespace util

package fileignorescope

// The same namespace as util.go, but a file-level ignore belongs to the file
// that carries it, so this one is still asked for its label.
func extra() int { return 2 } // want `func extra does not carry the prefix of namespace "util"; rename it to utilExtra`

var _ = extra
