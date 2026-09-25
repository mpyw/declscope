package b

import _ "unsafe"

//go:linkname pm example.com/review/internal/a.(*P).M
func pm()

var _ = pm
