//go:build never

package rx

import _ "unsafe"

var clash = 1

//go:linkname Linked example.com/rename/internal/rx.pushed
