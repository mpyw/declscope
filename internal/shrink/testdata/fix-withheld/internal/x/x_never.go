//go:build never

package x

import "io"

var _ = WinRef

var _ io.Closer = newOwn()
