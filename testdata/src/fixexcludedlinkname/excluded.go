//go:build declscope_never

package fixexcludedlinkname

import _ "unsafe"

// No build configuration the analysis runs under includes this file. The
// directive names helper in a comment, and the file writes no identifier
// that names it.

//go:linkname helper
